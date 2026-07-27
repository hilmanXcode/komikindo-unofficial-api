package model_komik

type Komik struct {
	Title  string
	Url    string
	ImgUrl string
}

type KomikChapter struct {
	Title      string
	UrlChapter string
}
