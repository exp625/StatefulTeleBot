package stb

import (
	"errors"
	"log"
	"sync"
)


// ErrEventRejected is the error returned when the state machine cannot process
// an event in the state that it is in.
var ErrEventRejected = errors.New("event rejected")

// Machine represents the state machine.
type Machine struct {
	// Current represents the current state.
	current StateType
	who *User
	ctx interface{}

	// states holds the configuration of states and events handled by the state machine.
	states map[StateType]*State
	globalEvents map[EventType]StateType

	// mutex ensures that only 1 event is processed by the state machine at any given time.
	mutex sync.Mutex
}

// getNextState returns the next state for the event given the machine's current
// state, or an error if the event can't be handled in the given state.
func (m *Machine) getNextState(event EventType) (StateType, error) {
	if state, ok := m.states[m.current]; ok {
		if state.Events != nil {
			if next, ok := state.Events[event]; ok {
				return next, nil
			}
		}
	}

	if m.globalEvents != nil {
		if next, ok := m.globalEvents[event]; ok {
			return next, nil
		}
	}

	return Default, ErrEventRejected
}

// SendEvent sends an event to the state machine.
func (m *Machine) SendEvent(event EventType) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	// Determine the next state for the event given the machine's current state.
	nextState, err := m.getNextState(event)
	if err != nil {
		return ErrEventRejected
	}

	// Identify the state definition for the next state.
	state, ok := m.states[nextState]
	if !ok || state.action == nil {
		// configuration error
	}
	log.Println(nextState)
	// Transition over to the next state.
	m.current = nextState
	if state.action != nil {
		action, ok := state.action.(func(*Machine))
		if !ok {
			panic("stb: action is bad")
		}

		state.runHandler(func() { action(m) })
	}

	return nil
}

func (m *Machine) User() *User {
	return m.who
}

func (m *Machine) Get() interface{} {
	return m.ctx
}

func (m *Machine) Set(ctx interface{})  {
	m.ctx = ctx
}

func (m *Machine) Current() StateType {
	return m.current
}
