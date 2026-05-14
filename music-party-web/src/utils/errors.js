export function extractErrorMessage(error, fallback = 'Request failed') {
  if (!error) return fallback;

  const responseMessage = error.response?.data?.message;
  if (typeof responseMessage === 'string' && responseMessage.trim()) {
    return responseMessage.trim();
  }

  if (typeof error.message === 'string' && error.message.trim()) {
    return error.message.trim();
  }

  return fallback;
}
