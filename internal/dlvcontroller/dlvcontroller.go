package dlvcontroller

type DlvController interface {
	StartSession() error
	SendCommand(command string) error
	ReceiveResponse() (string, error)
	QuitSession() error
}


