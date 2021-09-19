package stb

import (
	"fmt"
	"strings"
)

type StateType string

const Default StateType = "Default"

type State struct {
	Me       *User
	Type     StateType
	handlers map[string]interface{}
	Events   map[EventType]StateType
	action   interface{}

	synchronous bool
	verbose     bool
	reporter    func(error)
}

func (s *State) Handle(endpoint interface{}, handler interface{}) {
	switch end := endpoint.(type) {
	case string:
		s.handlers[end] = handler
	case CallbackEndpoint:
		s.handlers[end.CallbackUnique()] = handler
	default:
		panic("stb: unsupported endpoint")
	}
}

func (s *State) Action(handler interface{}) {
	s.action = handler
}

func (s *State) Event(e EventType, t StateType) {
	s.Events[e] = t
}

func (s State) processUpdate(upd Update, m *Machine) bool {

	if upd.Message != nil {
		msh := upd.Message

		if msh.PinnedMessage != nil {
			return s.handle(OnPinned, msh, m)
		}

		// Commands
		if msh.Text != "" {
			// Filtering malicious messages
			if msh.Text[0] == '\a' {
				return false
			}

			match := cmdRx.FindAllStringSubmatch(msh.Text, -1)
			if match != nil {
				// Syntax: "</command>@<bot> <payload>"

				command, botName := match[0][1], match[0][3]
				if botName != "" && !strings.EqualFold(s.Me.Username, botName) {
					return false
				}

				msh.Payload = match[0][5]
				if s.handle(command, msh, m) {
					return true
				}
			}

			// 1:1 satisfaction
			if s.handle(msh.Text, msh, m) {
				return true
			}

			if msh.Text[0] == '/' {
				return s.handle(OnCommand, msh, m)
			}

			return s.handle(OnText, msh, m)

		}

		if s.handleMedia(msh, m) {
			return true
		}

		if msh.Invoice != nil {
			return s.handle(OnInvoice, msh, m)

		}

		if msh.Payment != nil {
			return s.handle(OnPayment, msh, m)

		}

		wasAdded := (msh.UserJoined != nil && msh.UserJoined.ID == s.Me.ID) ||
			(msh.UsersJoined != nil && isUserInList(s.Me, msh.UsersJoined))
		if msh.GroupCreated || msh.SuperGroupCreated || wasAdded {
			return s.handle(OnAddedToGroup, msh, m)

		}

		if msh.UsersJoined != nil {
			b := false
			for index := range msh.UsersJoined {
				// Shallow copy message to prevent data race in async mode
				mm := *msh
				mm.UserJoined = &msh.UsersJoined[index]
				if s.handle(OnUserJoined, &mm, m) {
					b = true
				}
			}
			return b
		}

		if msh.UserJoined != nil {
			return s.handle(OnUserJoined, msh, m)

		}

		if msh.UserLeft != nil {
			return s.handle(OnUserLeft, msh, m)

		}

		if msh.NewGroupTitle != "" {
			return s.handle(OnNewGroupTitle, msh, m)

		}

		if msh.NewGroupPhoto != nil {
			return s.handle(OnNewGroupPhoto, msh, m)

		}

		if msh.GroupPhotoDeleted {
			return s.handle(OnGroupPhotoDeleted, msh, m)

		}

		if msh.MigrateTo != 0 {
			if handler, ok := s.handlers[OnMigration]; ok {
				handler, ok := handler.(func(int64, int64))
				if !ok {
					panic("stb: migration handler is bad")
				}

				s.runHandler(func() { handler(msh.Chat.ID, msh.MigrateTo) })
				return true
			}

			return false
		}

		if msh.VoiceChatStarted != nil {
			if handler, ok := s.handlers[OnVoiceChatStarted]; ok {
				handler, ok := handler.(func(*Message))
				if !ok {
					panic("stb: voice chat started handler is bad")
				}

				s.runHandler(func() { handler(msh) })
				return true
			}

			return false
		}

		if msh.VoiceChatEnded != nil {
			if handler, ok := s.handlers[OnVoiceChatEnded]; ok {
				handler, ok := handler.(func(*Message))
				if !ok {
					panic("stb: voice chat ended handler is bad")
				}

				s.runHandler(func() { handler(msh) })
				return true
			}

			return false
		}

		if msh.VoiceChatParticipantsInvited != nil {
			if handler, ok := s.handlers[OnVoiceChatParticipantsInvited]; ok {
				handler, ok := handler.(func(*Message))
				if !ok {
					panic("stb: voice chat participants invited handler is bad")
				}

				s.runHandler(func() { handler(msh) })
				return true
			}

			return false
		}

		if msh.ProximityAlert != nil {
			if handler, ok := s.handlers[OnProximityAlert]; ok {
				handler, ok := handler.(func(*Message))
				if !ok {
					panic("stb: proximity alert handler is bad")
				}

				s.runHandler(func() { handler(msh) })
				return true
			}

			return false
		}

		if msh.AutoDeleteTimer != nil {
			if handler, ok := s.handlers[OnAutoDeleteTimer]; ok {
				handler, ok := handler.(func(*Message))
				if !ok {
					panic("stb: auto delete timer handler is bad")
				}

				s.runHandler(func() { handler(msh) })
				return true
			}

			return false
		}

		if msh.VoiceChatSchedule != nil {
			if handler, ok := s.handlers[OnVoiceChatScheduled]; ok {
				handler, ok := handler.(func(*Message))
				if !ok {
					panic("stb: voice chat scheduled is bad")
				}

				s.runHandler(func() { handler(msh) })
				return true
			}

			return false
		}
	}

	if upd.EditedMessage != nil {
		return s.handle(OnEdited, upd.EditedMessage, m)

	}

	if upd.ChannelPost != nil {
		msg := upd.ChannelPost

		if msg.PinnedMessage != nil {
			return s.handle(OnPinned, msg, m)

		}

		return s.handle(OnChannelPost, upd.ChannelPost, m)

	}

	if upd.EditedChannelPost != nil {
		return s.handle(OnEditedChannelPost, upd.EditedChannelPost, m)

	}

	if upd.Callback != nil {
		if upd.Callback.Data != "" {
			if upd.Callback.MessageID != "" {
				upd.Callback.Message = &Message{
					// InlineID indicates that message
					// is inline so we have only its id
					InlineID: upd.Callback.MessageID,
				}
			}

			data := upd.Callback.Data
			if data[0] == '\f' {
				match := cbackRx.FindAllStringSubmatch(data, -1)
				if match != nil {
					unique, payload := match[0][1], match[0][3]

					if handler, ok := s.handlers["\f"+unique]; ok {
						handler, ok := handler.(func(*Callback, *Machine))
						if !ok {
							panic(fmt.Errorf("stb: %s callback handler is bad", unique))
						}

						upd.Callback.Data = payload
						s.runHandler(func() { handler(upd.Callback, m) })

						return true
					}
				}
			}
		}

		if handler, ok := s.handlers[OnCallback]; ok {
			handler, ok := handler.(func(*Callback, *Machine))
			if !ok {
				panic("stb: callback handler is bad")
			}

			s.runHandler(func() { handler(upd.Callback, m) })
			return true
		}

		return false
	}

	if upd.Query != nil {
		if handler, ok := s.handlers[OnQuery]; ok {
			handler, ok := handler.(func(*Query, *Machine))
			if !ok {
				panic("stb: query handler is bad")
			}

			s.runHandler(func() { handler(upd.Query, m) })
			return true
		}

		return false
	}

	if upd.ChosenInlineResult != nil {
		if handler, ok := s.handlers[OnChosenInlineResult]; ok {
			handler, ok := handler.(func(*ChosenInlineResult, *Machine))
			if !ok {
				panic("stb: chosen inline result handler is bad")
			}

			s.runHandler(func() { handler(upd.ChosenInlineResult, m) })
			return true
		}

		return false
	}

	if upd.ShippingQuery != nil {
		if handler, ok := s.handlers[OnShipping]; ok {
			handler, ok := handler.(func(*ShippingQuery, *Machine))
			if !ok {
				panic("stb: shipping query handler is bad")
			}

			s.runHandler(func() { handler(upd.ShippingQuery, m) })
			return true
		}

		return false
	}

	if upd.PreCheckoutQuery != nil {
		if handler, ok := s.handlers[OnCheckout]; ok {
			handler, ok := handler.(func(*PreCheckoutQuery, *Machine))
			if !ok {
				panic("stb: pre checkout query handler is bad")
			}

			s.runHandler(func() { handler(upd.PreCheckoutQuery, m) })
			return true
		}

		return false
	}

	if upd.Poll != nil {
		if handler, ok := s.handlers[OnPoll]; ok {
			handler, ok := handler.(func(*Poll))
			if !ok {
				panic("stb: poll handler is bad")
			}

			s.runHandler(func() { handler(upd.Poll) })
			return true
		}

		return false
	}

	if upd.PollAnswer != nil {
		if handler, ok := s.handlers[OnPollAnswer]; ok {
			handler, ok := handler.(func(*PollAnswer, *Machine))
			if !ok {
				panic("stb: poll answer handler is bad")
			}

			s.runHandler(func() { handler(upd.PollAnswer, m) })
			return true
		}

		return false
	}

	if upd.MyChatMember != nil {
		if handler, ok := s.handlers[OnMyChatMember]; ok {
			handler, ok := handler.(func(*ChatMemberUpdated, *Machine))
			if !ok {
				panic("stb: my chat member handler is bad")
			}

			s.runHandler(func() { handler(upd.MyChatMember, m) })
			return true
		}

		return false
	}

	if upd.ChatMember != nil {
		if handler, ok := s.handlers[OnChatMember]; ok {
			handler, ok := handler.(func(*ChatMemberUpdated, *Machine))
			if !ok {
				panic("stb: chat member handler is bad")
			}

			s.runHandler(func() { handler(upd.ChatMember, m) })
			return true
		}

		return false
	}
	return false
}

func (s *State) runHandler(handler func()) {
	f := func() {
		defer s.deferDebug()
		handler()
	}
	if s.synchronous {
		f()
	} else {
		go f()
	}
}

func (s *State) handle(end string, msg *Message, m *Machine) bool {

	if handler, ok := s.handlers[end]; ok {
		handler, ok := handler.(func(*Message, *Machine))
		if !ok {
			panic(fmt.Errorf("stb: %s handler is bad", end))
		}
		s.runHandler(func() { handler(msg, m) })

		return true
	}

	return false
}

func (s *State) handleMedia(msg *Message, m *Machine) bool {
	switch {
	case msg.Photo != nil:
		s.handle(OnPhoto, msg, m)
	case msg.Voice != nil:
		s.handle(OnVoice, msg, m)
	case msg.Audio != nil:
		s.handle(OnAudio, msg, m)
	case msg.Animation != nil:
		s.handle(OnAnimation, msg, m)
	case msg.Document != nil:
		s.handle(OnDocument, msg, m)
	case msg.Sticker != nil:
		s.handle(OnSticker, msg, m)
	case msg.Video != nil:
		s.handle(OnVideo, msg, m)
	case msg.VideoNote != nil:
		s.handle(OnVideoNote, msg, m)
	case msg.Contact != nil:
		s.handle(OnContact, msg, m)
	case msg.Location != nil:
		s.handle(OnLocation, msg, m)
	case msg.Venue != nil:
		s.handle(OnVenue, msg, m)
	case msg.Dice != nil:
		s.handle(OnDice, msg, m)
	default:
		return false
	}
	return true
}
