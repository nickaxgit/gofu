package tcs

type Tcs struct {
	Left, Top, Right, Bottom float64
}

func NewTcs(left, bottom, right, top float64) Tcs {
	return Tcs{Left: left, Top: top, Right: right, Bottom: bottom}
}
func (i *Tcs) Clone() *Tcs {
	return &Tcs{Left: i.Left, Top: i.Top, Right: i.Right, Bottom: i.Bottom}
}

func (i *Tcs) FlipV() *Tcs {
	return &Tcs{Left: i.Left, Top: i.Bottom, Right: i.Right, Bottom: i.Top}
}

func (i *Tcs) FlipH() *Tcs {
	return &Tcs{Left: i.Right, Top: i.Top, Right: i.Left, Bottom: i.Bottom}
}

func (tcs *Tcs) Width() float64 {
	return tcs.Right - tcs.Left
}

func (tcs *Tcs) Height() float64 {
	return tcs.Top - tcs.Bottom
}
