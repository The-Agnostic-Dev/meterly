package web

// Pager splits a long list into pages.
type Pager struct {
	Page    int
	PerPage int
	Total   int
}

// NewPager clamps page into range.
func NewPager(total, page, perPage int) Pager {
	p := Pager{Page: page, PerPage: perPage, Total: total}
	if p.PerPage < 1 {
		p.PerPage = 20
	}
	if p.Page > p.Pages() {
		p.Page = p.Pages()
	}
	if p.Page < 1 {
		p.Page = 1
	}
	return p
}

// Pages is the number of pages, at least 1.
func (p Pager) Pages() int {
	if p.Total == 0 {
		return 1
	}
	return (p.Total + p.PerPage - 1) / p.PerPage
}

// Window returns the zero-based positions of the first and last item on the page.
func (p Pager) Window() (first, last int) {
	first = (p.Page - 1) * p.PerPage
	last = min(first+p.PerPage, p.Total) - 1
	return first, last
}

// From and To are the 1-based positions shown as "Showing 11 to 20 of 52".
func (p Pager) From() int { first, _ := p.Window(); return first + 1 }
func (p Pager) To() int   { _, last := p.Window(); return last + 1 }

// HasPrev and HasNext drive the page links.
func (p Pager) HasPrev() bool { return p.Page > 1 }
func (p Pager) HasNext() bool { return p.Page < p.Pages() }
func (p Pager) Prev() int     { return p.Page - 1 }
func (p Pager) Next() int     { return p.Page + 1 }
