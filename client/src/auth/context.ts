import { createContext } from 'react'
import type { Genre, User } from '../api/types'

export interface RegisterInput {
  first_name: string
  last_name: string
  email: string
  password: string
  favorite_genres: Genre[]
}

export interface AuthState {
  user: User | null
  status: 'loading' | 'authenticated' | 'anonymous'
  login: (email: string, password: string) => Promise<void>
  register: (input: RegisterInput) => Promise<void>
  logout: () => Promise<void>
}

export const AuthContext = createContext<AuthState | null>(null)