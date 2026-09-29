package reviewfiles

type Files struct {
	GitCmd func(dir string, args ...string) (int, string, string)
}
