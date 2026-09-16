package jobs

type ReadableArgs struct {
	URL    string `json:"url"`
	Format string `json:"format"` // "pdf", "epub", "md", "html"
}

func (ReadableArgs) Kind() string { return "readable" }
