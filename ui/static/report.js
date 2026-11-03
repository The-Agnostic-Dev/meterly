// Click a column heading on the usage report to sort by it. Number columns
// sort highest first on the first click, text columns A to Z.
const table = document.querySelector('table.sortable')

if (table) {
  const body = table.tBodies[0]
  const headers = [...table.tHead.rows[0].cells]

  const sortBy = (th, index) => {
    const current = th.getAttribute('aria-sort')
    const descending = current ? current === 'ascending' : th.classList.contains('num')
    headers.forEach((h) => h.removeAttribute('aria-sort'))
    th.setAttribute('aria-sort', descending ? 'descending' : 'ascending')

    const rows = [...body.rows]
    rows.sort((a, b) => {
      const x = a.cells[index].textContent.trim()
      const y = b.cells[index].textContent.trim()
      return descending ? y.localeCompare(x) : x.localeCompare(y)
    })
    rows.forEach((row) => body.appendChild(row))
  }

  headers.forEach((th, index) => {
    th.tabIndex = 0
    th.classList.add('sortable-head')
    th.addEventListener('click', () => sortBy(th, index))
    th.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') sortBy(th, index)
    })
  })
}
