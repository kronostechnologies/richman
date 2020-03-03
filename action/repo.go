package action

type Chart struct {
	name string
	version string
}

type Repository struct {
	name string
	charts map[string]*Chart
}