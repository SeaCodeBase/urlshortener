import { NextRequest } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:8081'

async function proxyRequest(request: NextRequest, params: Promise<{ path: string[] }>) {
  const { path } = await params
  const url = new URL(request.url)
  const backendPath = `/api/${path.join('/')}${url.search}`

  const headers: HeadersInit = {
    'Content-Type': request.headers.get('Content-Type') || 'application/json',
  }

  const auth = request.headers.get('Authorization')
  if (auth) {
    headers['Authorization'] = auth
  }

  const fetchOptions: RequestInit = {
    method: request.method,
    headers,
  }

  // Include body for methods that support it
  if (['POST', 'PUT', 'PATCH'].includes(request.method)) {
    fetchOptions.body = await request.text()
  }

  const response = await fetch(`${BACKEND_URL}${backendPath}`, fetchOptions)

  // Return the response with the same status and headers
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: {
      'Content-Type': response.headers.get('Content-Type') || 'application/json',
    },
  })
}

export async function GET(request: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  return proxyRequest(request, params)
}

export async function POST(request: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  return proxyRequest(request, params)
}

export async function PUT(request: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  return proxyRequest(request, params)
}

export async function DELETE(request: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  return proxyRequest(request, params)
}

export async function PATCH(request: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  return proxyRequest(request, params)
}
