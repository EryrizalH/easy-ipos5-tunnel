package appcore

func StartService(name string) error {
	return startService(name)
}

func StopService(name string) error {
	return stopService(name)
}

func RestartService(name string) error {
	return restartService(name)
}
