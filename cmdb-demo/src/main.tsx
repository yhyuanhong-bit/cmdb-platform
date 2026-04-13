import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { Toaster } from 'sonner'
import App from './App'
import { LocationProvider } from './contexts/LocationContext'
import QueryProvider from './providers/QueryProvider'
import WebSocketProvider from './providers/WebSocketProvider'
import './i18n'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryProvider>
      <BrowserRouter>
        <WebSocketProvider>
          <LocationProvider>
            <App />
            <Toaster position="top-right" richColors />
          </LocationProvider>
        </WebSocketProvider>
      </BrowserRouter>
    </QueryProvider>
  </React.StrictMode>,
)
