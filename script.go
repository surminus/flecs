package main

// Script runs an arbitary command
type Script struct {
	Description string `yaml:"description"`
	Name        string `yaml:"name"`
	Path        string `yaml:"path"`
}
