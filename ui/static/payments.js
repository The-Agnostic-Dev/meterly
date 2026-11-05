// Records a service charge payment without leaving the invoice page.
const form = document.getElementById('payment-form')

if (form) {
  const message = document.getElementById('payment-message')
  const button = form.querySelector('button[type="submit"]')

  const show = (text, kind) => {
    message.className = 'status-message ' + kind
    message.textContent = text
  }

  const updateSummary = (data) => {
    document.getElementById('paid').textContent = data.paid
    document.getElementById('balance').textContent = data.balance
    const status = document.getElementById('status')
    status.textContent = data.status
    status.className = 'pill ' + data.status
  }

  form.addEventListener('submit', async (event) => {
    event.preventDefault()
    button.disabled = true
    show('Saving...', '')
    try {
      const res = await fetch(form.action, {
        method: 'POST',
        body: new FormData(form),
        headers: { Accept: 'application/json' },
      })
      const data = await res.json()
      if (data.ok) {
        updateSummary(data)
        show('Payment recorded. Balance is now ' + data.balance + '.', 'ok')
        form.querySelector('#payment-amount').value = ''
      } else {
        show(data.error, 'error')
      }
    } catch (err) {
      show('Could not reach Meterly. Check your connection and try again.', 'error')
    } finally {
      button.disabled = false
    }
  })
}
