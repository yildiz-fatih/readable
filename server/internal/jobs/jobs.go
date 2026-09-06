package jobs

type ReadableArgs struct {
	URL    string `json:"url"`
	Format string `json:"format"` // "html", "pdf", "epub"
}

func (ReadableArgs) Kind() string { return "readable" }
