package model_komik

type Komik struct {
	Title  string `json:"title"`
	ImgUrl string `json:"imgurl"`
	Slug   string `json:"slug"`
}

type KomikChapter struct {
	Title       string `json:"title"`
	SlugChapter string `json:"slugchapter"`
}

type KomikPanel struct {
	PanelNumber int    `json:"panelnumber"`
	ImgUrl      string `json:"imgurl"`
}
