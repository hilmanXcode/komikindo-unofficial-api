package model_komik

type Komik struct {
	Id     int
	Title  string `json:"title"`
	ImgUrl string `json:"imgurl"`
	Slug   string `json:"slug"`
}

type KomikChapter struct {
	Id          int
	Title       string `json:"title"`
	SlugChapter string `json:"slugchapter"`
	SlugKomik   string `json:"slugkomik"`
}

type KomikPanel struct {
	Id          int
	SlugChapter string
	PanelNumber int    `json:"panelnumber"`
	ImgUrl      string `json:"imgurl"`
}
