package app

import "github.com/hjhsamuel/agent/config"

func Start() error {
	if err := config.Init(); err != nil {
		return err
	}

	return nil
}
