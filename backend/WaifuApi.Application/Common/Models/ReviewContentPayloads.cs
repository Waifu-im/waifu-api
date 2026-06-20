using System.Collections.Generic;

namespace WaifuApi.Application.Common.Models;

/// <summary>
/// Full editable content of a pending NewContent submission (what the edit modal sends). Deserialized with
/// <see cref="EditPayloadOptions"/> (strict), so privileged fields like reviewStatus/uploaderId are rejected.
/// </summary>
public class NewImageContent
{
    public string? Source { get; set; }
    public bool? IsNsfw { get; set; }
    public List<string>? Tags { get; set; }   // full set of tag slugs
    public List<long>? Artists { get; set; }  // full set of artist ids
}

public class NewTagContent
{
    public string? Name { get; set; }
    public string? Description { get; set; }
}

public class NewArtistContent
{
    public string? Name { get; set; }
    public string? Patreon { get; set; }
    public string? Pixiv { get; set; }
    public string? Twitter { get; set; }
    public string? DeviantArt { get; set; }
}
