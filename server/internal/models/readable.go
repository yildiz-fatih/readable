package models

import "time"

type ReadableFormat string

const (
	HTML ReadableFormat = "html"
	PDF  ReadableFormat = "pdf"
	EPUB ReadableFormat = "epub"
)

type ReadableStatus string

const (
	Pending   ReadableStatus = "pending"
	Succeeded ReadableStatus = "succeeded"
	Failed    ReadableStatus = "failed"
)

type Readable struct {
	ID      int
	Status  ReadableStatus
	Format  ReadableFormat
	Created time.Time
}
