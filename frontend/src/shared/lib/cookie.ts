import Cookies from 'js-cookie'

const Cookie = {
  get(key: string): string | undefined {
    return Cookies.get(key)
  },

  set(key: string, value: string, options?: Cookies.CookieAttributes): void {
    Cookies.set(key, value, {
      secure: process.env.NODE_ENV === 'production',
      sameSite: 'strict',
      ...options,
    })
  },

  remove(key: string): void {
    Cookies.remove(key)
  },
}

export default Cookie
