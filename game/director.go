package game

type Director interface {
	Start(*Board)
	End()
}
