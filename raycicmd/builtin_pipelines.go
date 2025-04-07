package raycicmd

// builtin ray buildkite pipeline IDs.
const (
	// v1 pipelines
	rayBranchPipeline = "0183465b-c6fb-479b-8577-4cfd743b545d"
	rayPRPipeline     = "0183465f-a222-467a-b122-3b9ea3e68094"

	// v2 pipelines
	rayV2PostmergePipeline  = "0189e759-8c96-4302-b6b5-b4274406bf89"
	rayV2PremergePipeline   = "0189942e-0876-4b8f-80a4-617f988ec59b"
	rayV2MicrocheckPipeline = "018f4f1e-1b73-4906-9802-92422e3badaa"

	// dev only
	rayDevPipeline = "5b097a97-ad35-4443-9552-f5c413ead11c"

	// pipeline for this repo
	rayCIPipeline = "01890992-4f1a-4cc4-b99d-f2da360eb3ab"
)

const (
	rayCIECR = "029272617770.dkr.ecr.us-west-2.amazonaws.com"

	rayBazelBuildCache = "https://bazel-cache-dev.s3.us-west-2.amazonaws.com"

	defaultForgePrefix = "cr.ray.io/rayproject/"
)
