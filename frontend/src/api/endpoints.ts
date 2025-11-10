import { api } from "./client"

export type ShortenRequest = {
    url: string
}

export type ShortenResponse = {
    short_url: string | null
    original_url: string
    error: string | null
}

export async function shortenURL(data: ShortenRequest): Promise<ShortenResponse> {
    const res = await api.post<ShortenResponse>("/shorten", data)
    return res.data
}