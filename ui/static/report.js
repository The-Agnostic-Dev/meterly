// Click a column heading on the usage report to sort by it. Number columns
// sort highest first on the first click, text columns A to Z.
const table = document.querySelector('table.sortable')

if (table) {
  const body = table.tBodies[0]
  const headers = [...table.tHead.rows[0].cells]

  const parseValue = (text) => {
    const value = parseFloat(text.replace(/[₦,]/g, ''))
    return Number.isNaN(value) ? null : value
  }

  const isNumber = (text) => {
    const cleaned = text.replace(/kWh/g, '').replace(/[₦,\s]/g, '')
    return /^[-+]?\d+(?:\.\d+)?$/.test(cleaned)
  }

  const sortBy = (th, index) => {
    const rows = [...body.rows]
    const numeric =
      th.classList.contains('num') ||
      (rows.length > 0 && rows.every((row) => isNumber(row.cells[index].textContent.trim())))

    const current = th.getAttribute('aria-sort')
    const descending = current ? current === 'ascending' : numeric
    headers.forEach((h) => h.removeAttribute('aria-sort'))
    th.setAttribute('aria-sort', descending ? 'descending' : 'ascending')

    rows.sort((a, b) => {
      const x = a.cells[index].textContent.trim()
      const y = b.cells[index].textContent.trim()
      if (!numeric) {
        if (x === y) return 0
        if (descending) return x < y ? 1 : -1
        return x < y ? -1 : 1
      }

      const nx = parseValue(x)
      const ny = parseValue(y)
      if (nx === null && ny === null) return 0
      if (nx === null) return 1
      if (ny === null) return -1
      return descending ? ny - nx : nx - ny
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
