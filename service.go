package main

// ServiceStep will update a Service
type ServiceStep struct {
	Definition  string `yaml:"definition"`
	Description string `yaml:"description"`
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
}
