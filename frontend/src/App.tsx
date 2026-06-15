import { createSignal, onCleanup, ParentComponent, Show } from 'solid-js'
import Nav from './components/Nav'

const App: ParentComponent = (props) => {
  const [visible, setVisible] = createSignal(false)

  const onScroll = () => setVisible(window.scrollY > 400)
  window.addEventListener('scroll', onScroll, { passive: true })
  onCleanup(() => window.removeEventListener('scroll', onScroll))

  return (
    <>
      <header class="nav-wrapper">
        <Nav />
      </header>
      <div class="app">
        <main class="container">
          {props.children}
        </main>
      </div>
      <Show when={visible()}>
        <button
          class="scroll-top-btn"
          onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
          aria-label="Scroll to top"
        >
          ↑
        </button>
      </Show>
    </>
  )
}

export default App
