import { useMutation } from '@tanstack/react-query';
import { api } from '@/lib/api';

/** Uploads an image file → `{url, name, size, content_type}`. */
export function useUploadImage() {
  return useMutation({
    mutationFn: (file: File) => api.uploadImage(file),
  });
}
