// Small enhancements shared by every page. Pages work without them.

function showReadingForm() {
  const form = document.getElementById('new-reading-form')
  form.hidden = false
  form.querySelector('input[name="value"]').focus()
}

document.querySelectorAll('[data-action="add-reading"]').forEach((button) => {
  button.addEventListener('click', showReadingForm)
})

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
