package storage_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/synclog"
)

// --- the transport ---

// blobSource is the exchange as one device sees it: blobs still downloading
// are absent and a damaged read is served once.
type blobSource struct {
	s *faultSim
	d *simDevice
}

func (b blobSource) Blob(digest string) ([]byte, error) {
	if due, lagging := b.d.blobDue[digest]; lagging && b.s.tick < due {
		b.s.count("blob_unavailable_reads")
		return nil, fmt.Errorf("blob %s is still downloading", digest)
	}
	data, ok := b.s.ex.blobs[digest]
	if !ok {
		return nil, fmt.Errorf("blob %s is not in the exchange", digest)
	}
	if b.d.corruptBlob[digest] {
		delete(b.d.corruptBlob, digest)
		b.s.count("blob_corrupt_reads")
		damaged := append([]byte{}, data...)
		damaged[len(damaged)/2] ^= 0x20
		return damaged, nil
	}
	return data, nil
}

func manifestDigests(m *synclog.Manifest) []string {
	var out []string
	for _, effect := range m.Effects {
		for _, digest := range []*string{effect.BaseBlob, effect.Blob} {
			if digest != nil {
				out = append(out, *digest)
			}
		}
	}
	return out
}

// publish copies every outgoing transaction of d, and every blob it names,
// into the exchange, queues it for every other device, and then confirms the
// publication, possibly crashing while it does.
func (s *faultSim) publish(d *simDevice) {
	if d.offline || d.observer {
		return
	}
	pending, err := d.history.Pending()
	if err != nil {
		s.fatalf("%s: pending: %v", d.name, err)
	}
	if len(pending) == 0 {
		return
	}
	ids := make([]string, 0, len(pending))
	for _, m := range pending {
		data, encodeErr := m.Encode()
		if encodeErr != nil {
			s.fatalf("%s: encode: %v", d.name, encodeErr)
		}
		for _, digest := range manifestDigests(m) {
			blob, blobErr := d.history.Blob(digest)
			if blobErr != nil {
				s.fatalf("%s: outgoing %s names blob %s it does not retain: %v", d.name, m.TxnID, digest, blobErr)
			}
			s.ex.blobs[digest] = blob
		}
		if previous, seen := s.ex.manifests[m.TxnID]; seen {
			if !bytes.Equal(previous, data) {
				s.fatalf("%s republished %s with different bytes", d.name, m.TxnID)
			}
			s.count("republished")
			s.logf("%s republish %s seq %d", d.name, short(m.TxnID), m.DeviceSeq)
		} else {
			s.ex.manifests[m.TxnID] = data
			s.ex.position[m.TxnID] = len(s.ex.order)
			s.ex.order = append(s.ex.order, m.TxnID)
			s.count("published")
			s.logf("%s publish %s seq %d kind %s parents %d effects %d", d.name, short(m.TxnID), m.DeviceSeq, m.Kind, len(m.Parents), len(m.Effects))
		}
		for _, other := range s.devices {
			if other != d {
				s.enqueue(other, m, data)
			}
		}
		ids = append(ids, m.TxnID)
	}
	before := len(pending)
	crashed := s.withCrash(d, "publication confirmation", func() error { return d.svc.CommitSyncPublished(ids) })
	after, err := d.history.Pending()
	if err != nil {
		s.fatalf("%s: pending: %v", d.name, err)
	}
	switch {
	case !crashed && len(after) != 0:
		s.fatalf("%s: %d transactions still pending after confirming publication", d.name, len(after))
	case crashed && len(after) != 0 && len(after) != before:
		s.fatalf("%s: a crashed confirmation drained %d of %d transactions", d.name, before-len(after), before)
	}
}

// enqueue sends a published manifest toward d through the faulty transport.
func (s *faultSim) enqueue(d *simDevice, m *synclog.Manifest, data []byte) {
	delay := 0
	if s.cfg.MaxDelay > 0 {
		delay = s.rng.Intn(s.cfg.MaxDelay + 1)
	}
	if delay > 0 {
		s.count("delayed")
	}
	if s.chance(s.cfg.Truncate) {
		s.push(d, m.TxnID, data[:1+s.rng.Intn(len(data)-3)], "truncated", s.tick+s.rng.Intn(delay+1))
	}
	if s.chance(s.cfg.Corrupt) {
		damaged := append([]byte{}, data...)
		damaged[s.rng.Intn(len(damaged))] ^= byte(1 + s.rng.Intn(255))
		s.push(d, m.TxnID, damaged, "corrupted", s.tick+s.rng.Intn(delay+1))
	}
	s.push(d, m.TxnID, data, "", s.tick+delay)
	if s.chance(s.cfg.Duplicate) {
		extra := 0
		if s.cfg.MaxDelay > 0 {
			extra = s.rng.Intn(s.cfg.MaxDelay + 1)
		}
		s.push(d, m.TxnID, data, "duplicate", s.tick+extra)
	}
	for _, digest := range manifestDigests(m) {
		if _, lagging := d.blobDue[digest]; lagging {
			continue
		}
		if s.chance(s.cfg.BlobLag) {
			d.blobDue[digest] = s.tick + 1 + s.rng.Intn(s.cfg.MaxDelay+2)
			s.count("blob_lagged")
		}
		if s.chance(s.cfg.CorruptBlob) {
			d.corruptBlob[digest] = true
			s.count("blob_corrupted")
		}
	}
}

func (s *faultSim) push(d *simDevice, txnID string, data []byte, fault string, due int) {
	s.queued++
	if fault != "" {
		s.count("sent_" + fault)
	}
	d.inbox = append(d.inbox, &delivery{txnID: txnID, data: data, fault: fault, due: due, order: s.queued})
}

// drain delivers, in a seeded order, everything due at d.
func (s *faultSim) drain(d *simDevice) {
	if d.offline {
		return
	}
	var ready, later []*delivery
	for _, del := range d.inbox {
		if del.due <= s.tick {
			ready = append(ready, del)
		} else {
			later = append(later, del)
		}
	}
	d.inbox = later
	s.rng.Shuffle(len(ready), func(i, j int) { ready[i], ready[j] = ready[j], ready[i] })
	for _, del := range ready {
		s.deliver(d, del)
	}
}

// requeue retries a delivery the device could not apply yet.
func (s *faultSim) requeue(d *simDevice, del *delivery) {
	s.push(d, del.txnID, s.ex.manifests[del.txnID], "", s.tick+1)
}

// deliver hands one manifest file to d, holding every outcome to its
// invariant: a damaged file is refused and changes nothing, a repeat changes
// nothing, an incomplete one changes nothing and is retried, and an apply is
// atomic even when the process dies in the middle of it.
func (s *faultSim) deliver(d *simDevice, del *delivery) {
	m, err := synclog.DecodeManifest(del.data)
	if err != nil {
		if del.fault == "" {
			s.fatalf("%s: a complete manifest %s does not decode: %v", d.name, short(del.txnID), err)
		}
		if !errors.Is(err, synclog.ErrInvalidManifest) && !errors.Is(err, synclog.ErrHeld) {
			s.fatalf("%s: a %s manifest was refused as %v", d.name, del.fault, err)
		}
		s.count("refused_" + del.fault)
		s.logf("%s refuse %s copy of %s", d.name, del.fault, short(del.txnID))
		return
	}
	if m.TxnID != del.txnID {
		s.fatalf("%s: a %s copy of %s decoded as a different valid transaction %s", d.name, del.fault, short(del.txnID), short(m.TxnID))
	}
	if del.fault == "truncated" || del.fault == "corrupted" {
		// Damage to insignificant whitespace leaves the same transaction.
		s.count("harmless_" + del.fault)
	}
	before := s.fingerprint(d)
	var crashed bool
	err = nil
	crashed = s.withCrash(d, "incoming "+short(m.TxnID), func() error {
		_, err = d.svc.CommitIncoming(m, blobSource{s: s, d: d})
		return err
	})
	if crashed {
		s.requeue(d, del)
		return
	}
	switch {
	case err == nil:
		s.count("applied")
		position := s.ex.position[m.TxnID]
		if position < d.applied {
			s.count("applied_out_of_order")
		}
		d.applied = max(d.applied, position)
		s.logf("%s apply %s from %s seq %d", d.name, short(m.TxnID), m.DeviceID, m.DeviceSeq)
	case errors.Is(err, synclog.ErrAlreadyApplied), errors.Is(err, synclog.ErrOwnTransaction):
		s.requireUnchanged(d, before, "a repeated delivery")
		s.count("repeat_ignored")
		s.logf("%s ignore repeat of %s", d.name, short(m.TxnID))
	case errors.Is(err, synclog.ErrIncomplete):
		s.requireUnchanged(d, before, "an incomplete delivery")
		if strings.Contains(err.Error(), "blob") {
			s.count("waiting_for_blob")
		} else {
			s.count("waiting_for_ancestor")
		}
		s.logf("%s wait %s: %v", d.name, short(m.TxnID), err)
		s.requeue(d, del)
	default:
		s.fatalf("%s: apply %s from %s seq %d: %v", d.name, short(m.TxnID), m.DeviceID, m.DeviceSeq, err)
	}
}

func (s *faultSim) requireUnchanged(d *simDevice, before, what string) {
	if after := s.fingerprint(d); after != before {
		s.fatalf("%s: %s changed the device:\n%s\n---\n%s", d.name, what, before, after)
	}
}

// withCrash runs a commit, and with the configured probability stops its
// journal transaction at a seeded point and restarts the device's process.
// It reports whether the commit was stopped.
func (s *faultSim) withCrash(d *simDevice, what string, fn func() error) bool {
	if !s.chance(s.cfg.Crash) {
		if err := fn(); err != nil && !isRefusal(err) {
			s.fatalf("%s: %s: %v", d.name, what, err)
		}
		return false
	}
	prefix, ops, crashed, err := storage.CrashCommitForTest(s.t, d.txDir(), func(ops int) int { return s.rng.Intn(ops+2) - 1 }, fn)
	if !crashed {
		if err != nil && !isRefusal(err) {
			s.fatalf("%s: %s: %v", d.name, what, err)
		}
		return false
	}
	s.count("crashes")
	s.logf("%s crash during %s after %d of %d operations", d.name, what, prefix, ops)
	s.restart(d, "crash during "+what)
	return true
}

// isRefusal is an incoming manifest the device declines without applying it.
func isRefusal(err error) bool {
	return errors.Is(err, synclog.ErrAlreadyApplied) || errors.Is(err, synclog.ErrOwnTransaction) || errors.Is(err, synclog.ErrIncomplete)
}

func short(txnID string) string {
	hex := strings.TrimPrefix(txnID, "sha256:")
	if len(hex) > 10 {
		return hex[:10]
	}
	return hex
}

// --- driving runs ---

// randomStep is one seeded event: a local edit, a publication, a delivery
// round, a restart, or a device going offline or online.
func (s *faultSim) randomStep() {
	s.tick++
	authors := s.authors()
	d := authors[s.rng.Intn(len(authors))]
	if s.chance(s.cfg.Restart) {
		s.restart(d, "relaunch")
	}
	if s.chance(s.cfg.Offline) {
		d.offline = !d.offline
		s.count("offline_toggles")
		s.logf("%s offline=%v", d.name, d.offline)
	}
	switch op := s.rng.Intn(14); op {
	case 0, 1:
		s.retitleRandom(d)
	case 2:
		s.capture(d)
	case 3:
		s.deleteLeaf(d)
	case 4:
		s.reprioritize(d)
	case 5:
		s.plan(d)
	case 6:
		s.appendNote(d)
	case 7:
		if !s.resolveRandom(d) {
			s.retitleRandom(d)
		}
	case 8, 9, 10:
		s.publish(d)
	default:
		s.drain(d)
	}
}

// settle brings every device online and exchanges until every outbox and
// inbox is empty, crashes and transport faults included.
func (s *faultSim) settle() {
	s.logf("settle")
	for _, d := range s.devices {
		d.offline = false
	}
	for round := 0; ; round++ {
		if round > 400 {
			s.fatalf("the exchange did not settle")
		}
		s.tick++
		busy := false
		for _, d := range s.devices {
			s.publish(d)
		}
		for _, d := range s.devices {
			s.drain(d)
		}
		for _, d := range s.devices {
			pending, err := d.history.Pending()
			if err != nil {
				s.fatalf("%s: pending: %v", d.name, err)
			}
			if len(d.inbox) > 0 || len(pending) > 0 {
				busy = true
			}
		}
		if !busy {
			return
		}
	}
}

// addObservers clones devices that start from genesis and receive the whole
// exchange through the same faulty transport, each in its own seeded order.
func (s *faultSim) addObservers(n int) {
	for i := 0; i < n; i++ {
		o := s.addDevice(fmt.Sprintf("observer-%d", i), true)
		order := append([]string{}, s.ex.order...)
		if i%2 == 1 {
			for a, b := 0, len(order)-1; a < b; a, b = a+1, b-1 {
				order[a], order[b] = order[b], order[a]
			}
		} else {
			s.rng.Shuffle(len(order), func(a, b int) { order[a], order[b] = order[b], order[a] })
		}
		for _, txnID := range order {
			s.enqueue(o, s.decode(txnID), s.ex.manifests[txnID])
		}
		s.logf("observer-%d joins and receives %d transactions", i, len(order))
	}
}

func (s *faultSim) decode(txnID string) *synclog.Manifest {
	m, err := synclog.DecodeManifest(s.ex.manifests[txnID])
	if err != nil {
		s.fatalf("exchange holds an invalid manifest %s: %v", short(txnID), err)
	}
	return m
}

// finish settles, adds genesis observers, settles again, and holds every
// replica to convergence, preservation, and idempotence. It returns the
// conflicts every replica agrees on.
func (s *faultSim) finish(observers int) []synclog.Conflict {
	s.settle()
	s.addObservers(observers)
	s.settle()
	conflicts := s.requireConverged()
	s.requirePreserved(conflicts)
	s.requireIdempotent()
	for _, d := range s.devices {
		s.restart(d, "final relaunch")
		report, err := d.svc.CommitExternalChanges()
		if err != nil {
			s.fatalf("%s: observe after relaunch: %v", d.name, err)
		}
		if report.Imported != nil || len(report.Held) > 0 {
			s.fatalf("%s: the converged workspace holds changes its history never recorded: %+v", d.name, report)
		}
	}
	s.logf("finished: %d transactions, %d open conflicts; %s", len(s.ex.order), len(conflicts), s.summary())
	return conflicts
}
