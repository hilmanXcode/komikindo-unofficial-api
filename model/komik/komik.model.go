package model_komik

type Komik struct {
	Title  string
	ImgUrl string
	Slug   string
}

type KomikChapter struct {
	Title       string
	SlugChapter string
}

type KomikPanel struct {
	PanelNumber int
	ImgUrl      string
}
