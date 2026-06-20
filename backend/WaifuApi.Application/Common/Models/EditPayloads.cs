using System.Collections.Generic;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace WaifuApi.Application.Common.Models;

/// <summary>
/// Serialization settings for edit payloads.
/// <list type="bullet">
/// <item>camelCase + case-insensitive (Web defaults) to match the frontend.</item>
/// <item>Null properties are omitted on write, so a stored payload only contains the fields the user changed.</item>
/// <item>Unknown members are rejected on read — this is the security boundary that prevents a client from
/// smuggling privileged fields (e.g. reviewStatus, uploaderId, slug) into the payload.</item>
/// </list>
/// </summary>
public static class EditPayloadOptions
{
    public static readonly JsonSerializerOptions Strict = new(JsonSerializerDefaults.Web)
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
        UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow
    };
}

/// <summary>
/// Proposed changes to an Image. A null property means "leave unchanged". For <see cref="Source"/>, an empty
/// string means "clear". Tag/artist membership is expressed as deltas so independent additions by different
/// users don't conflict.
/// </summary>
public class ImageEditPayload
{
    public string? Source { get; set; }
    public bool? IsNsfw { get; set; }
    public List<string>? AddTagSlugs { get; set; }
    public List<string>? RemoveTagSlugs { get; set; }
    public List<long>? AddArtistIds { get; set; }
    public List<long>? RemoveArtistIds { get; set; }
}

/// <summary>
/// Proposed changes to a Tag. A null property means "leave unchanged".
/// Slug is intentionally not editable via edit requests (it is a public API identifier).
/// </summary>
public class TagEditPayload
{
    public string? Name { get; set; }
    public string? Description { get; set; }
}

/// <summary>
/// Proposed changes to an Artist. A null property means "leave unchanged"; an empty string clears the link.
/// </summary>
public class ArtistEditPayload
{
    public string? Name { get; set; }
    public string? Patreon { get; set; }
    public string? Pixiv { get; set; }
    public string? Twitter { get; set; }
    public string? DeviantArt { get; set; }
}
