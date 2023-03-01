package config

import (
	"debridGo/types"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

func ExecutableDir() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", err
	}
	exPath := filepath.Dir(ex)

	return exPath, err
}

func Values() (types.TomlConfig, error) {

	var conf types.TomlConfig

	// Get executable path.
	ex, err := ExecutableDir()
	if err != nil {
		return conf, err
	}

	_, err = toml.DecodeFile(ex+"/configDebridGo.toml", &conf)
	// _, err := toml.DecodeFile("configDebridGo.toml", &conf)
	if err != nil {
		return conf, err
	}

	return conf, nil
}

func SetFfmpeg(value bool) error {
	var conf types.TomlConfig

	conf.Ffmpeg.Running = value

	// Get executable path.
	ex, err := ExecutableDir()
	if err != nil {
		return err
	}

	f, err := os.Open(ex + "/configDebridGo.toml")
	if err != nil {
		// failed to create/open the file
		return err
	}
	if err := toml.NewEncoder(f).Encode(conf); err != nil {
		// failed to encode
		return err
	}
	if err := f.Close(); err != nil {
		// failed to close the file
		return err
	}

	return nil
}
