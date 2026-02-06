namespace WaifuApi.Web.Models;

/// <summary>
/// Standard error response following RFC 9457 Problem Details.
/// </summary>
/// <example>
/// {
///   "type": "https://tools.ietf.org/html/rfc9110#section-15.5.5",
///   "title": "Resource Not Found",
///   "status": 404,
///   "detail": "The image with ID '123' was not found.",
///   "instance": "/images/123",
///   "traceId": "00-abc123def456-789xyz-00"
/// }
/// </example>
public class ErrorResponse
{
    /// <summary>
    /// A URI reference that identifies the problem type.
    /// </summary>
    /// <example>https://tools.ietf.org/html/rfc9110#section-15.5.5</example>
    public string? Type { get; set; }

    /// <summary>
    /// A short, human-readable summary of the problem type.
    /// </summary>
    /// <example>Resource Not Found</example>
    public string? Title { get; set; }

    /// <summary>
    /// The HTTP status code.
    /// </summary>
    /// <example>404</example>
    public int? Status { get; set; }

    /// <summary>
    /// A human-readable explanation specific to this occurrence of the problem.
    /// </summary>
    /// <example>The image with ID '123' was not found.</example>
    public string? Detail { get; set; }

    /// <summary>
    /// A URI reference that identifies the specific occurrence of the problem.
    /// </summary>
    /// <example>/images/123</example>
    public string? Instance { get; set; }

    /// <summary>
    /// A unique identifier for this request, useful for debugging.
    /// </summary>
    /// <example>00-abc123def456-789xyz-00</example>
    public string? TraceId { get; set; }
}

/// <summary>
/// Validation error response with field-level errors.
/// </summary>
/// <example>
/// {
///   "type": "https://tools.ietf.org/html/rfc9110#section-15.5.1",
///   "title": "One or more validation errors occurred.",
///   "status": 400,
///   "detail": "See errors property for details.",
///   "instance": "/images",
///   "traceId": "00-abc123def456-789xyz-00",
///   "errors": {
///     "name": ["The Name field is required.", "Name must be at least 3 characters."],
///     "url": ["The URL format is invalid."]
///   }
/// }
/// </example>
public class ValidationErrorResponse : ErrorResponse
{
    /// <summary>
    /// A dictionary containing field names as keys and arrays of error messages as values.
    /// </summary>
    /// <example>
    /// {
    ///   "name": ["The Name field is required.", "Name must be at least 3 characters."],
    ///   "url": ["The URL format is invalid."]
    /// }
    /// </example>
    public Dictionary<string, string[]>? Errors { get; set; }
}
