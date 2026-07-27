package keymap

import tea "charm.land/bubbletea/v2"

// ModalGuard intercepts key events before direct keymap dispatch.
type ModalGuard struct {
	Active func() bool
	Handle func(key string) (tea.Model, tea.Cmd, bool)
}

// Dispatcher routes direct key events through a KeyMap.
type Dispatcher struct {
	keymap        *KeyMap
	ModalGuards   []ModalGuard
	operationExec OperationExecutor
}

// OperationExecutor invokes one stable operation or declines so Dispatcher can
// run the binding's retained action.
type OperationExecutor func(operationID, shortcut string, fallback Action) (tea.Model, tea.Cmd, bool)

func NewDispatcher(keymap *KeyMap) *Dispatcher {
	return &Dispatcher{keymap: keymap}
}

func (dispatcher *Dispatcher) SetOperationExecutor(executor OperationExecutor) {
	dispatcher.operationExec = executor
}

// Dispatch processes one direct key press.
func (dispatcher *Dispatcher) Dispatch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	key := msg.String()
	for _, guard := range dispatcher.ModalGuards {
		if guard.Active == nil || !guard.Active() || guard.Handle == nil {
			continue
		}
		if model, cmd, handled := guard.Handle(key); handled {
			return model, cmd, true
		}
	}
	if dispatcher.keymap == nil {
		return nil, nil, false
	}
	for _, entry := range dispatcher.keymap.Entries {
		binding := entry.Binding
		if binding == nil || binding.Key != key {
			continue
		}
		if binding.Gate != nil && !binding.Gate() {
			continue
		}
		if binding.Action == nil {
			return nil, nil, true
		}
		if binding.OperationID != "" && dispatcher.operationExec != nil {
			if model, cmd, handled := dispatcher.operationExec(binding.OperationID, key, binding.Action); handled {
				return model, cmd, true
			}
		}
		model, cmd := binding.Action()
		return model, cmd, true
	}
	return nil, nil, false
}
