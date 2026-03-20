export function createTricklePublisher(url, mimeType) {
  let idx = 0
  let nextController = null

  // kick off the create POST
  async function create() {
    const resp = await fetch(url, {
      method: 'POST',
      headers: { 'Expect-Content': mimeType },
    })
    if (!resp.ok) {
      const msg = await resp.text()
      throw new Error(`Trickle create failed: ${resp.status} ${msg}`)
    }
  }

  // open a streamed POST for segment `i`
  async function preconnect(i) {
    let controller
    const stream = new ReadableStream({
      start(ctrl) { controller = ctrl },
      cancel(reason) {
        console.warn(`segment ${i} cancelled:`, reason)
      },
    })

    fetch(`${url}/${i}`, {
      method: 'POST',
      headers: { 'Content-Type': mimeType, 'Transfer-Encoding': 'chunked' },
      body: stream,
      duplex: 'half',
    }).then(r => {
        if (!r.ok) r.text().then(t => console.error(`seg ${i} err:`, r.status, t))
    }).catch(e => console.error(`seg ${i} POST failed:`, e))

    return controller
  }

  // get the next SegmentWriter
  async function next() {
    // first‐time or after each .next()
    if (nextController === null) {
      nextController = await preconnect(idx)
    }

    const seq = idx++
    const ctrl = nextController
    nextController = null

    // fire off the subsequent preconnect
    preconnect(idx).then(c => (nextController = c)).catch(() => {})

    return {
      write(chunk) {
        /*
        // accept string or ArrayBuffer/TypedArray
        if (typeof chunk === 'string') {
          ctrl.enqueue(new TextEncoder().encode(chunk))
        } else */{
          ctrl.enqueue(chunk)
        }
      },
      close() {
        ctrl.close()
      },
      seq: () => seq,
    }
  }

  // close any in-flight segment and DELETE the stream
  async function close() {
    if (nextController) {
      nextController.close()
      nextController = null
    }
    fetch(url, { method: 'DELETE' }).catch(e => console.error('DELETE failed:', e))
  }

  return { create, next, close }
}
