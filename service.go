package main

// Service will update one or many services and wait for completion
type Service struct {
	Definition  string `yaml:"definition"`
	Description string `yaml:"description"`
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
}
