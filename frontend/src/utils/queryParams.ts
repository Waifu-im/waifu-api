type ParamValue = string | number | boolean | undefined | null;

interface BuildQueryParamsOptions {
  /** Values to convert to query params (single values) */
  params?: Record<string, ParamValue>;
  /** Arrays to append as multiple values with the same key */
  arrays?: Record<string, (string | number)[] | undefined>;
}

/**
 * Build URLSearchParams from an object, filtering out undefined/null values
 * and handling arrays that should be appended with the same key.
 *
 * @example
 * const params = buildQueryParams({
 *   params: { page: 1, isNsfw: 'True', name: undefined },
 *   arrays: { includedTags: ['tag1', 'tag2'], excludedTags: [] }
 * });
 * // Result: "page=1&isNsfw=True&includedTags=tag1&includedTags=tag2"
 */
export function buildQueryParams(options: BuildQueryParamsOptions): URLSearchParams {
  const searchParams = new URLSearchParams();

  // Handle single-value params
  if (options.params) {
    for (const [key, value] of Object.entries(options.params)) {
      if (value !== undefined && value !== null && value !== '') {
        searchParams.append(key, String(value));
      }
    }
  }

  // Handle array params (append each value with the same key)
  if (options.arrays) {
    for (const [key, values] of Object.entries(options.arrays)) {
      if (values && values.length > 0) {
        for (const value of values) {
          searchParams.append(key, String(value));
        }
      }
    }
  }

  return searchParams;
}
