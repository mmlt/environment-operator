package infra

// SourceStep gets the terraform configuration and expands templates
type SourceStep struct {
	StepMeta
	SHA string
}

func (st SourceStep) Name() string {
	return "source"
}
