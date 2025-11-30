package catalog

type App struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Author   Author `json:"author"`
	Download string `json:"download"`
}

type Author struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
