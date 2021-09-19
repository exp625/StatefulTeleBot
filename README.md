# StatefulTeleBot

StatefulTeleBot is a Telegram Bot framework based on the awesome [telebot](https://github.com/tucnak/telebot) package,
that was modified to be stateful. Before stating to use this package, please read the documentation
for [telebot](https://github.com/tucnak/telebot).

# Changes between Telebot and Statefultelebot

This project aims to keep the original public API of the telebot package, however to make the bot stateful a few changes
were introduced

## New Handler signature

All Handlers require a second parameter by which the current State Machine is passed to the handler. If a Handler can't
recognize a Request, the State Machine can be nil. See Recognizer for more information.

```go
b.Handle(tb.OnText, func (msg *stb.Message, m *stb.Machine) {

})

```

## Added and modified endpoints

Added stb.OnCommand endpoints and stb.OnText no longer captures commands

```go
b.Handle(tb.OnCommand, func (msg *stb.Message, m *stb.Machine) {
// all the commands that weren't
// captured by existing handlers
})

```

# Getting Started

Let's create a statefultelebot setup, where you can use the bot like a telebot instance.

```go
package main

import (
	"github.com/exp625/stb"
	"log"
	"time"
)

func main() {
	b, err := stb.NewBot(stb.Settings{
		// Add default recognizer function
		Recognizer: stb.DefaultRecognizer,
		Token:      "TOKEN_HERE",
		Poller:     &stb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	// Important: Define the Default State!
	const DefaultState stb.StateType = "DefaultState"
	b.Default(DefaultState)

	b.Handle("/hello", func(msg *stb.Message, m *stb.Machine) {
		b.Send(msg.Sender, "Hello World from the "+string(m.Current())+" State!")
	})

	b.Start()
}
```

Okay, but let's take a look at some example using the stateful feature of the statefultelebot instance

## Basic states example with actions

Here we create two states:

* InactiveState which is registered as the default state
* ActiveState on which a Handler is registered, that responds to send text messages

Using the commands ``/activate`` and ``/deactivate`` the user can switch between the two states using the tow registered
events. The handlers of the commands are registered on the stb instance itself and are therefore available regardless of
the current state.

```go
package main

import (
	"github.com/exp625/stb"
	"log"
	"time"
)

func main() {
	b, err := stb.NewBot(stb.Settings{
		Recognizer: stb.DefaultRecognizer,
		Token:      "TOKEN_HERE",
		Poller:     &stb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	// Define States
	const Inactive stb.StateType = "Inactive"
	const Active stb.StateType = "Active"
	inactive := b.Default(Inactive)
	active := b.State(Active)

	// Define Events
	const Activate stb.EventType = "Activate"
	const Deactivate stb.EventType = "Deactivate"
	inactive.Event(Activate, Active)
	active.Event(Deactivate, Inactive)

	//Define Actions
	inactive.Action(func(m *stb.Machine) {
		b.Send(m.User(), "Entered the inactive state")
	})

	active.Action(func(m *stb.Machine) {
		b.Send(m.User(), "Entered the active state")
	})

	// Define commands globally
	b.Handle("/activate", func(msg *stb.Message, m *stb.Machine) {
		m.SendEvent(Activate)
	})
	b.Handle("/deactivate", func(msg *stb.Message, m *stb.Machine) {
		m.SendEvent(Deactivate)
	})

	// Register handler only on the active state
	// Only respond to text messages in the active state
	active.Handle(stb.OnText, func(msg *stb.Message, m *stb.Machine) {
		b.Send(msg.Sender, msg.Text)
	})

	b.Start()
}
```

This will result in the following chat example:

```
User:   Hello
User:   /activate
                                Entered the active state    :Bot
User:   Hello
                                Hello                       :Bot
User:   /deactivate
                                Entered the inactive state  :Bot
User:   Hello
```

## Using the State Context to pass information

Here we build a simple user registration flow, where the entered information is passed along using the State Context

```go
package main

import (
	"github.com/exp625/stb"
	"log"
	"time"
)

const DefaultState stb.StateType = "DefaultState"
const EnterName stb.StateType = "EnterName"
const EnterEmail stb.StateType = "EnterEmail"
const SaveConfirmation stb.StateType = "SaveConfirmation"
const Next stb.EventType = "Next"
const Cancel stb.EventType = "Cancel"

var (
	keyboard = &stb.ReplyMarkup{}
	btnYes   = keyboard.Data("Yes", "yesBtn")
	btnNo    = keyboard.Data("No", "noBtn")
)

type User struct {
	Name  string
	Email string
}

func main() {
	b, err := stb.NewBot(stb.Settings{
		Recognizer: stb.DefaultRecognizer,
		Token:      "TOKEN_HERE",
		Poller:     &stb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	// Respond to all Callbacks as a default
	b.Handle(stb.OnCallback, func(c *stb.Callback, m *stb.Machine) {
		b.Respond(c, &stb.CallbackResponse{})
	})
	// Add global Cancel event that will always switch back to the default state and cancel the registration
	defaultState := b.Default(DefaultState)
	b.Event(Cancel, stb.Default)
	b.Handle("/cancel", func(msg *stb.Message, m *stb.Machine) {
		b.Send(msg.Sender, "Canceled registration")
		m.Set(nil)
		m.SendEvent(Cancel)
	})
	// Add start command to the default state and pass empty user struct to the context
	defaultState.Event(Next, EnterName)
	defaultState.Handle("/start", func(msg *stb.Message, m *stb.Machine) {
		user := User{}
		m.Set(user)
		m.SendEvent(Next)
	})
	// Listen to text messages and save the name
	enterName := b.State(EnterName)
	enterName.Event(Next, EnterEmail)
	enterName.Action(func(m *stb.Machine) {
		b.Send(m.User(), "Please enter your Name")
	})
	enterName.Handle(stb.OnText, func(msg *stb.Message, m *stb.Machine) {
		user := m.Get().(User)
		user.Name = msg.Text
		m.Set(user)
		m.SendEvent(Next)
	})
	// Listen to text messages and save the email
	enterMail := b.State(EnterEmail)
	enterMail.Event(Next, SaveConfirmation)
	enterMail.Action(func(m *stb.Machine) {
		b.Send(m.User(), "Please enter your Email")
	})
	enterMail.Handle(stb.OnText, func(msg *stb.Message, m *stb.Machine) {
		user := m.Get().(User)
		user.Email = msg.Text
		m.Set(user)
		m.SendEvent(Next)
	})
	// Display the user information and aks for conformation with an inline keyboard
	saveConfirmation := b.State(SaveConfirmation)
	saveConfirmation.Event(Next, stb.Default)
	saveConfirmation.Action(func(m *stb.Machine) {
		user := m.Get().(User)
		text := "Save the following information? \n Name: " + user.Name + "\n Email: " + user.Email
		keyboard.Inline(keyboard.Row(btnYes, btnNo))
		b.Send(m.User(), text, keyboard)
	})
	// Register handlers for the inline buttons
	saveConfirmation.Handle(&btnYes, func(c *stb.Callback, m *stb.Machine) {
		b.Respond(c, &stb.CallbackResponse{})
		b.Send(c.Sender, "Saved your information")
		// Save information to db, etc.
		m.Set(nil)
		m.SendEvent(Next)
	})
	saveConfirmation.Handle(&btnNo, func(c *stb.Callback, m *stb.Machine) {
		b.Respond(c, &stb.CallbackResponse{})
		b.Send(c.Sender, "Canceled")
		m.Set(nil)
		m.SendEvent(Next)
	})

	b.Start()
}
```

# Added functions and types

Here is a description of the added functions of the statefultelebot package. Only new or changed methods are listed.
See [telebot](https://github.com/tucnak/telebot) for a full overview of available functions and types.

## ``RecognizerFunc(upd stb.Update) (*stb.User, error)``

The Recognizer function is used to map the state machine to the processed update. You can create your own recognizer
function or use the provided default function that will look for the ``stb.User`` in the ``stb.Update``

```go

func DefaultRecognizer(upd Update) (*User, error) {
if upd.Message != nil {
if upd.Message.Sender != nil {
return upd.Message.Sender, nil
}
}

if upd.Callback != nil {
if upd.Callback.Sender != nil {
return upd.Callback.Sender, nil
}
}

if upd.Query != nil {
return &upd.Query.From, nil
}

if upd.ChosenInlineResult != nil {
return &upd.ChosenInlineResult.From, nil
}

if upd.ShippingQuery != nil {
if upd.ShippingQuery.Sender != nil {
return upd.ShippingQuery.Sender, nil
}
}

if upd.PreCheckoutQuery != nil {
if upd.PreCheckoutQuery.Sender != nil {
return upd.PreCheckoutQuery.Sender, nil
}
}

if upd.PollAnswer != nil {
return &upd.PollAnswer.User, nil
}

if upd.MyChatMember != nil {
return &upd.MyChatMember.From, nil
}

if upd.ChatMember != nil {
return &upd.ChatMember.From, nil
}

return nil, errors.New("No ID for update")
}
```

You can pass your own recognizer function when creation the ``stb.Bot`` instance

```go
b, err := stb.NewBot(stb.Settings{
Recognizer: customRecognizerFunction,
Token:  "TOKEN_HERE",
Poller: &stb.LongPoller{Timeout: 10 * time.Second},
})
```

## ``stb.StateType``

The ``StateType`` defines an available state for the bot

```go
const DefaultState StateType = "DefaultState"
```

## ``stb.EventType``

The ``EventType`` defines an available event for the bot

```go
const CancelEvent EventType = "CancelEvent"
```

## ``stb.Bot.Default(state StateType) *State``

The ``Bot.Default`` function is used to create the default state. When a new user starts using the bot, this is the
state that he starts in. The bot will not start without defining a default state.

```go
defaultState := b.Default(DefaultState)
```

## ``stb.Bot.State(state StateType) *State``

The ``Bot.State`` function is used to create a new state, that the bot can enter.

```go
activeState := b.State(ActiveState)
```

## ``stb.State.Event(event stb.EventType, state stb.StateType)``

Register an Event for a State. When the event get executed the state machine transitions into the passed state.

```go
defaultState.Event(Activate, ActiveState)
```

## ``stb.Bot.Event(event EventType)``

Register an Event globally. The event will be available for all states.

```go
b.Event(CancelCommand, DefaultState)
```

## ``stb.State.Handle(endpoint string, handlerFunc func(*stb.Message, *stb.Machine))``

Register a handler to a state for the specified endpoint. The handler will only be used, when the bot is in the current
state.

```go
defaultState.Handle(stb.OnText, func (msg *stb.Message, m *stb.Machine) {
b.Send(msg.Sender, msg.Text)
})
```

## ``stb.Bot.Handle(endpoint string, handlerFunc func(*stb.Message, *stb.Machine))``

Register a handler globally. If no state handler already handled an update, the global handler will be called.
IMPORTANT: If the recognizer function could not find a user for the update, the ``stb.Machine`` pointer will be nil

```go
b.Handle("/current", func (msg *stb.Message, m *stb.Machine) {
if m != nil {
b.Send(msg.Sender, "Current State:" + string(m.Current))
}	    
})
```

## ``stb.State.Action(actionFunc func(*stb.Machine))``

An action will be executed when the state machine enters the corresponding state.

```go
activeState.Action(func (m *stb.Machine) {
b.Send(m.User(), "Entered the active State")
})
```

## ``stb.Machine.SendEvent(event stb.EventType) error``

Send an Event to the state machine. If the event was registered to the current state of the state machine, the event is
executed and the state machine transitions into the specified state. If the event is not registered for the current
state an error is returned.

```go
b.SendEvent(CancelEvent)
```

## ``stb.Machine.User() *stb.User``

Return the User the state machine belongs to

```go
b.Send(m.User(), "Message")
```

## ``stb.Machine.Set(ctx interface{})``

Set the current Machine Context

```go
data := Data{}
m.Set(data)
```

## ``stb.Machine.Get() interface{}``

Get the current Machine Context

```go
data := m.Get().(Data)
```

## ``stb.Machine.Current() stb.StateType``

Return the current State of the Machine

```go
currentState := m.Current()
```

# Tips and Tricks

## Reuse the same keyboard

```go

var (
keyboard = &stb.ReplyMarkup{}
btnYes = keyboard.Data("Yes", "yesBtn")
btnNo = keyboard.Data("No", "noBtn")
)

/...

.../

stateA := b.State(StateA)
saveConfirmation.AddAction(func (m *stb.Machine) {
keyboard.Inline(keyboard.Row(btnYes, btnNo))
b.Send(m.User(), "Do you want to save the information", keyboard)
})
stateB := b.State(StateB)
saveConfirmation.AddAction(func (m *stb.Machine) {
keyboard.Inline(keyboard.Row(btnYes, btnNo))
b.Send(m.User(), "Do you want to cancel the registration", keyboard)
})

// Add two handlers for the same button on different states
stateA.Handle(&btnYes, func(c *stb.Callback, m *stb.Machine) {
b.Respond(c, &stb.CallbackResponse{})
b.Send(c.Sender, "Saved your information")
})
stateB.Handle(&btnYes, func(c *stb.Callback, m *stb.Machine) {
b.Respond(c, &stb.CallbackResponse{})
b.Send(c.Sender, "Canceled the registration")
})

```