package raycicmd

func makePipeline(repoDir string, config *config, buildID string) (
	*bkPipeline, error,
) {
	pl := new(bkPipeline)

	// TODO(aslonnie): build rayci pipeline here.

	totalSteps := 0
	for _, group := range pl.Steps {
		totalSteps += len(group.Steps)
	}
	if totalSteps == 0 {
		q, ok := config.RunnerQueues["default"]
		if !ok {
			q = ""
		}
		return makeNoopBkPipeline(q), nil
	}

	return pl, nil
}