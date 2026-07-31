package config

import "fmt"

func saveDomainBundle(store *DomainFileStore, domains *Domains) error {
	if domains == nil || domains.Catalog == nil || domains.Planning == nil || domains.Machine == nil {
		return fmt.Errorf("all configuration domains are required")
	}
	if err := store.SaveCatalog(domains.Catalog); err != nil {
		return err
	}
	if err := store.SavePlanning(domains.Planning); err != nil {
		return err
	}
	return store.SaveMachine(domains.Machine)
}
