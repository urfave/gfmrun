package gfmxr

type Runner struct {
	SourceFiles          []string
	ExpectedExampleCount int
}

func NewRunner(sourceFiles []string, expectedExampleCount int) *Runner {
	return &Runner{
		SourceFiles:          sourceFiles,
		ExpectedExampleCount: expectedExampleCount,
	}
}

func (r *Runner) Run() []error {
	return nil
}
