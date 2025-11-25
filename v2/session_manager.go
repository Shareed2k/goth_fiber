package goth_fiber

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
)

type sessionManager struct {
	session *session.Store
}

func NewSessionManager(s *session.Store) *sessionManager {
	// Create new storage handler
	sessionManager := new(sessionManager)
	if s != nil {
		// Use provided storage if provided
		sessionManager.session = s
	}

	return sessionManager
}

// get value from session
func (m *sessionManager) getValue(c fiber.Ctx, key string) (string, error) {
	sess := session.FromContext(c)
	var value string
	var ok bool

	if sess != nil {
		value, ok = sess.Get(key).(string)
	} else {
		// Try to get the session from the store
		storeSess, err := m.session.Get(c)
		if err != nil {
			// Handle error
			return "", err
		}

		defer storeSess.Release()

		value, ok = storeSess.Get(key).(string)
	}

	if ok {
		return value, nil
	}

	return "", errors.New("could not find a matching session for this request")
}

// set value in session
func (m *sessionManager) setValue(c fiber.Ctx, key string, value string) error {
	sess := session.FromContext(c)
	if sess != nil {
		sess.Set(key, value)
	} else {
		// Try to get the session from the store
		storeSess, err := m.session.Get(c)
		if err != nil {
			return err
		}

		defer storeSess.Release()

		storeSess.Set(key, value)
		if err := storeSess.Save(); err != nil {
			return err
		}
	}

	return nil
}

// delete session
func (m *sessionManager) delSession(c fiber.Ctx) error {
	sess := session.FromContext(c)
	if sess != nil {
		if err := sess.Destroy(); err != nil {
			return err
		}
	} else {
		// Try to get the session from the store
		storeSess, err := m.session.Get(c)
		if err != nil {
			return err
		}

		defer storeSess.Release()

		if err := storeSess.Destroy(); err != nil {
			return err
		}

		if err := storeSess.Save(); err != nil {
			return err
		}
	}

	return nil
}
