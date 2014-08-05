// +build appengine

package sendgrid

import (
	"reflect"
	"testing"

	aemail "appengine/mail"
)

func TestMigrate(t *testing.T) {
	message := NewMail()
	message.AddTo("john@email.com")
	message.AddToName("John Doe")
	message.SetSubject("test")
	message.SetHTML("html")
	message.SetText("text")
	message.SetFrom("doe@email.com")
	message.SetFromName("Doe Email")
	message.AddBcc("bcc@host.com")
	message.AddCc("cc@host.com")

	aemessage := aemail.Message{
		To:       []string{"John Doe <john@email.com>"},
		Sender:   "Doe Email <doe@email.com>",
		Subject:  "test",
		HTMLBody: "html",
		Body:     "text",
		Bcc:      []string{"bcc@host.com"},
		Cc:       []string{"cc@host.com"},
	}

	message2, err := migrateMail(&aemessage)
	if err != nil {
		t.Errorf("Error migrating mail - %v", err)
	}

	sg := NewSendGridClient("", "")
	val1, err := sg.buildURL(message)
	if err != nil {
		t.Errorf("%s", err)
	}

	val2, err := sg.buildURL(message2)
	if err != nil {
		t.Errorf("%s", err)
	}

	if !reflect.DeepEqual(message, message2) {
		t.Errorf("messages not equal\n%#v\n%#v", message, message2)
	}

	for key := range val1 { // make sure everything in val1 is in val2
		if val1.Get(key) != val2.Get(key) {
			t.Errorf("Expected %#v, got %#v", val1.Get(key), val2.Get(key))
		}
		val2.Del(key)
	}
	for key := range val2 { // make sure nothing in val2 isn't in val1
		if val1.Get(key) != val2.Get(key) {
			t.Errorf("Expected %#v, got %#v", val1.Get(key), val2.Get(key))
		}
	}
}
