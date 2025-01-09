package reefagent

type Command struct {
	Command string
	Args    []string
}

type Job struct {
	Id       string
	Commands []string
}
