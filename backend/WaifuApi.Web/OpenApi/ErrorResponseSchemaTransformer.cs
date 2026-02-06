using System.Text.Json.Nodes;
using Microsoft.AspNetCore.OpenApi;
using Microsoft.OpenApi;
using WaifuApi.Web.Models;

namespace WaifuApi.Web.OpenApi;

public class ErrorResponseSchemaTransformer : IOpenApiSchemaTransformer
{
    public Task TransformAsync(
        OpenApiSchema schema,
        OpenApiSchemaTransformerContext context,
        CancellationToken cancellationToken)
    {
        if (context.JsonTypeInfo.Type == typeof(ErrorResponse))
        {
            schema.Example = new JsonObject
            {
                ["type"] = "https://tools.ietf.org/html/rfc9110#section-15.5.5",
                ["title"] = "Resource Not Found",
                ["status"] = 404,
                ["detail"] = "The image with ID '123' was not found.",
                ["instance"] = "/images/123",
                ["traceId"] = "00-abc123def456-789xyz-00"
            };
        }
        else if (context.JsonTypeInfo.Type == typeof(ValidationErrorResponse))
        {
            schema.Example = new JsonObject
            {
                ["type"] = "https://tools.ietf.org/html/rfc9110#section-15.5.1",
                ["title"] = "One or more validation errors occurred.",
                ["status"] = 400,
                ["detail"] = "See errors property for details.",
                ["instance"] = "/images",
                ["traceId"] = "00-abc123def456-789xyz-00",
                ["errors"] = new JsonObject
                {
                    ["name"] = new JsonArray("The Name field is required.", "Name must be at least 3 characters."),
                    ["url"] = new JsonArray("The URL format is invalid.")
                }
            };
        }

        return Task.CompletedTask;
    }
}
