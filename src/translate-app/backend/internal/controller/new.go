package controller

import (
	"translate-app/config"
	"translate-app/internal/controller/file"
	"translate-app/internal/controller/message"
	"translate-app/internal/controller/session"
	"translate-app/internal/controller/settings"
	"translate-app/internal/logger"
	"translate-app/internal/repository"
)

// Controllers aggregates domain controllers for DI.
type Controllers struct {
	Session  session.Controller
	Message  message.Controller
	File     file.Controller
	Settings settings.Controller
}

// New wires all controllers. API keys are used by the message controller to build AI clients per request from settings + optional overrides.
func New(reg repository.Registry, keys *config.APIKeys, log logger.Logger) *Controllers {
	fileCtrl := file.New(reg, keys, log)
	msg := message.New(reg, keys, fileCtrl, log)
	return &Controllers{
		Session:  session.New(reg, msg),
		Message:  msg,
		File:     fileCtrl,
		Settings: settings.New(reg, log),
	}
}
