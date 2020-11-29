package main

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Log allows us to output in a consistent way everywhere
var Log = logrus.New()

// CheckError will display any errors and quit if found
func CheckError(err error) {
	if err != nil {
		Log.Error(err)
		os.Exit(1)
	}
}
