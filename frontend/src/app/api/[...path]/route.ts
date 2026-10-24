import { NextRequest } from 'next/server'
import { buildProxyRequestHeaders, buildProxyResponseHeaders } from '@/lib/proxyHeaders'

const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:8081'

async function proxyRequest(request: NextRequest, params: Promise<{ path: string[] }>) {
  const { path } = await params
  const url = new URL(request.url)
  const backendPath = `/api/${path.join('/')}${url.search}`

  const headers = buildProxyRequestHeaders(request.headers)

  const fetchOptions: RequestInit = {
    method: request.method,
    headers,
  }

  // Include body for methods that support it
  if (['POST', 'PUT', 'PATCH'].includes(request.method)) {
    fetchOptions.body = await request.text()
  }

  const response = await fetch(`${BACKEND_URL}${backendPath}`, fetchOptions)

  // Same status; only allowlisted headers (Content-Type, Content-Disposition).
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: buildProxyResponseHeaders(response.headers),
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
