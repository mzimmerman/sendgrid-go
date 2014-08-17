// +build appengine

package sendgrid

import (
	"errors"
	"fmt"
	netmail "net/mail"
	"strings"
	"sync"

	"appengine"
	"appengine/datastore"
	"appengine/delay"
	aemail "appengine/mail"
	"appengine/urlfetch"
)

var globalConfig *Config
var configSync sync.Mutex

var ErrConfig = errors.New("Unable to fetch SendGrid config")

var SendgridDelay = delay.Func("sendgrid", sendMail)

func sendMail(c appengine.Context, sgmail *SGMail) error {
	if appengine.IsDevAppServer() {
		c.Infof("Would have sent e-mail - %#v", sgmail)
		return nil
	}
	err := loadConfig(c)
	if err != nil {
		return err
	}
	sgclient := NewSendGridClient(globalConfig.APIUser, globalConfig.APIPassword)
	sgclient.Client = urlfetch.Client(c)
	return sgclient.Send(sgmail)
}

type Config struct {
	APIUser     string
	APIPassword string
}

func loadConfig(c appengine.Context) error {
	configSync.Lock()
	defer configSync.Unlock()
	if globalConfig == nil {
		key := datastore.NewKey(c, "SendGridConfig", "SendGridConfig", 0, nil)
		tempConfig := new(Config)
		err := datastore.Get(c, key, tempConfig)
		if err != nil {
			c.Errorf("Unable to fetch SendGrid config, please populate information in web console - %v", err)
			_, err = datastore.Put(c, key, &Config{
				APIUser:     "default",
				APIPassword: "default",
			})
			// put the default stub entry
			// so it can be updated in the web console
			if err != nil {
				c.Errorf("Error putting stub SendGrid config - %v", err)
			}
			return ErrConfig
		}
		if tempConfig.APIPassword == "default" || tempConfig.APIUser == "default" ||
			tempConfig.APIPassword == "" || tempConfig.APIUser == "" {
			c.Errorf("Found default SendGrid config in the datastore, please populate information in web console")
			return ErrConfig
		}
		globalConfig = tempConfig
	}
	return nil
}

func migrateMail(m *aemail.Message) (*SGMail, error) {
	sgmail := SGMail{
		Subject: m.Subject,
		HTML:    m.HTMLBody,
		Text:    m.Body,
		ReplyTo: m.ReplyTo,
	}
	if address, err := netmail.ParseAddress(m.Sender); err == nil {
		sgmail.SetFrom(address.Address)
		sgmail.SetFromName(address.Name)
	} else {
		return nil, fmt.Errorf("Error parsing Sender address - %v", err)
	}
	if addresses, err := netmail.ParseAddressList(strings.Join(m.To, ",")); err == nil {
		for _, addr := range addresses {
			sgmail.AddTo(addr.Address)
			sgmail.AddToName(addr.Name)
		}
	} else {
		return nil, fmt.Errorf("Error parsing To addresses - %v", err)
	}
	if len(m.Cc) > 0 {
		if addresses, err := netmail.ParseAddressList(strings.Join(m.Cc, ",")); err == nil {
			for _, addr := range addresses {
				sgmail.AddCc(addr.Address)
			}
		} else {
			return nil, fmt.Errorf("Error parsing CC - %v", err)
		}
	}
	if len(m.Bcc) > 0 {
		if addresses, err := netmail.ParseAddressList(strings.Join(m.Bcc, ",")); err == nil {
			for _, addr := range addresses {
				sgmail.AddBcc(addr.Address)
			}
		} else {
			return nil, fmt.Errorf("Error parsing BCC - %v", err)
		}
	}
	return &sgmail, nil
}

// SendMailDelay uses the appengine/delay package to add the sending of the message to the default task queue
// in the devappserver, it prints the output to the logs immediately
func SendMailDelay(c appengine.Context, m *aemail.Message) error {
	sgmail, err := migrateMail(m)
	if err != nil {
		return err
	}
	if appengine.IsDevAppServer() {
		// in the dev server, send it immediately
		return sendMail(c, sgmail)
	} else {
		SendgridDelay.Call(c, sgmail)
	}
	return nil
}
