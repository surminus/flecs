package main

func (config Config) Run() (err error) {
	for _, step := range config.Pipeline {
		Log.Info("Step: ", step.Type)

		switch step.Type {
		case "task":
			Log.Info("Name: ", step.Task.Name)
		case "service":
			Log.Info("Name: ", step.Service.Name)

			// serviceStep := step.Service
			// serviceStep.clientECS = ClientECS{}
			// serviceStep.config = config

		case "script":
			Log.Info("Name: ", step.Script.Name)
			_, err = step.Script.Run()
			if err != nil {
				return err
			}
		default:
			Log.Fatal("Invalid configuration")
		}
	}

	return err
}
