import axios from "axios";

export const api = axios.create({
    baseURL: import.meta.env.VITE_API_BASE_URL,
    timeout: 5000,
    headers: {
        "Content-Type": "application/json",
    },
})

api.interceptors.response.use(
    (response) => response,
    (error) => {
        console.error("API Error:", error)
        throw error
    }
)