import { downloadBlob } from '@/lib/download'

/** The exact header the bulk-upload parser expects (order fixed). */
export const BULK_TEMPLATE_HEADER = 'long_url,title,custom_back_half,tags,domain'

/** Header plus one example row; an empty domain means the group's default. */
export const BULK_TEMPLATE_CSV = `${BULK_TEMPLATE_HEADER}\nhttps://example.com/landing,Spring sale,spring-sale,promo|email,\n`

export function downloadBulkTemplate(): void {
  downloadBlob('bulk-upload-template.csv', new Blob([BULK_TEMPLATE_CSV], { type: 'text/csv' }))
}
