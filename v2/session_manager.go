package goth_fiber

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
)

type sessionManager struct {
	session *session.Store
}

// NewSessionManager wraps a Fiber session store for use by this package.
//
// s must not be nil. There is no default to fall back to here, so a nil store
// would only surface as a nil pointer dereference on the first request that
// touches the session.
func NewSessionManager(s *session.Store) *sessionManager {
	if s == nil {
		panic("goth_fiber: NewSessionManager requires a non-nil *session.Store")
	}

	return &sessionManager{session: s}
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
