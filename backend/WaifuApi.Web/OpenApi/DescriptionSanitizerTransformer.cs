using System.Net;
using Microsoft.AspNetCore.OpenApi;
using Microsoft.OpenApi;

namespace WaifuApi.Web.OpenApi;

/// <summary>
/// Decodes HTML entities in OpenAPI descriptions to fix display issues in Scalar.
/// </summary>
public class DescriptionSanitizerTransformer : IOpenApiDocumentTransformer
{
    public Task TransformAsync(OpenApiDocument document, OpenApiDocumentTransformerContext context, CancellationToken cancellationToken)
    {
        // Process all paths
        if (document.Paths == null) return Task.CompletedTask;

        foreach (var path in document.Paths)
        {
            if (path.Value?.Operations == null) continue;
            foreach (var operation in path.Value.Operations)
            {
                if (operation.Value == null) continue;

                // Decode operation description and summary
                if (operation.Value.Description != null)
                {
                    operation.Value.Description = DecodeHtmlEntities(operation.Value.Description);
                }
                if (operation.Value.Summary != null)
                {
                    operation.Value.Summary = DecodeHtmlEntities(operation.Value.Summary);
                }

                // Decode parameter descriptions
                if (operation.Value.Parameters != null)
                {
                    foreach (var parameter in operation.Value.Parameters)
                    {
                        if (parameter.Description != null)
                        {
                            parameter.Description = DecodeHtmlEntities(parameter.Description);
                        }
                    }
                }

                // Decode request body descriptions
                if (operation.Value.RequestBody?.Description != null)
                {
                    operation.Value.RequestBody.Description = DecodeHtmlEntities(operation.Value.RequestBody.Description);
                }

                // Decode response descriptions
                if (operation.Value.Responses != null)
                {
                    foreach (var response in operation.Value.Responses)
                    {
                        if (response.Value.Description != null)
                        {
                            response.Value.Description = DecodeHtmlEntities(response.Value.Description);
                        }
                    }
                }
            }
        }

        // Process tags descriptions
        if (document.Tags != null)
        {
            foreach (var tag in document.Tags)
            {
                if (tag.Description != null)
                {
                    tag.Description = DecodeHtmlEntities(tag.Description);
                }
            }
        }

        return Task.CompletedTask;
    }

    private static string DecodeHtmlEntities(string text)
    {
        return WebUtility.HtmlDecode(text);
    }
}
