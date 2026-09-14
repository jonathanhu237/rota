import { useTranslation as useI18nTranslation } from 'react-i18next'

/** Rota retains its established dot-key catalog while the Temvia shell owns the i18n instance. */
export function useRotaTranslation() {
  const result = useI18nTranslation<any>()
  return { ...result, t: result.t as any }
}
