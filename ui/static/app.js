// Small enhancements shared by every page. Pages work without them.

function showReadingForm() {
  const form = document.getElementById('new-reading-form')
  form.hidden = false
  form.querySelector('input[name="value"]').focus()
}

document.querySelectorAll('[data-action="add-reading"]').forEach((button) => {
  button.addEventListener('click', showReadingForm)
})

const readingForm = document.getElementById('new-reading-form')
if (readingForm) {
  readingForm.addEventListener('submit', (event) => {
    const valueInput = readingForm.querySelector('input[name="value"]')
    const value = parseFloat(valueInput.value.replace(/,/g, ''))
    const rows = document.querySelectorAll('#readings tbody tr[data-value]')
    if (Number.isNaN(value) || rows.length === 0) return
    const last = parseFloat(rows[0].dataset.value)
    if (Number.isNaN(last)) return
    if (value < last) {
      event.preventDefault()
      const error = document.getElementById('reading-error')
      if (error) {
        error.textContent = 'the reading cannot be lower than the last reading, check the figure on the meter'
        error.hidden = false
      }
    }
  })
}

function printPage() {
  window.print()
}

document.querySelectorAll('[data-action="print"]').forEach((button) => {
  button.addEventListener('click', printPage)
})

document.querySelectorAll('form[data-confirm]').forEach((form) => {
  form.addEventListener('submit', (event) => {
    if (!window.confirm(form.dataset.confirm)) event.preventDefault()
  })
})
