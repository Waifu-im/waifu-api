using System.ComponentModel;
using System.Reflection;
using Microsoft.AspNetCore.OpenApi;
using Microsoft.OpenApi;

namespace WaifuApi.Web.OpenApi;

/// <summary>
/// Reads [Description] attributes from properties and adds them to the OpenAPI schema.
/// </summary>
public class DescriptionSchemaTransformer : IOpenApiSchemaTransformer
{
    public Task TransformAsync(OpenApiSchema schema, OpenApiSchemaTransformerContext context, CancellationToken cancellationToken)
    {
        var type = context.JsonTypeInfo.Type;

        // Add description from [Description] attribute on the type itself
        var typeDescription = type.GetCustomAttribute<DescriptionAttribute>();
        if (typeDescription != null && string.IsNullOrEmpty(schema.Description))
        {
            schema.Description = typeDescription.Description;
        }

        // Add descriptions to properties from [Description] attributes
        if (schema.Properties != null)
        {
            foreach (var property in schema.Properties)
            {
                var propertyInfo = type.GetProperty(property.Key, BindingFlags.Public | BindingFlags.Instance | BindingFlags.IgnoreCase);
                if (propertyInfo != null)
                {
                    var descAttr = propertyInfo.GetCustomAttribute<DescriptionAttribute>();
                    if (descAttr != null && string.IsNullOrEmpty(property.Value.Description))
                    {
                        property.Value.Description = descAttr.Description;
                    }
                }
            }
        }

        return Task.CompletedTask;
    }
}
