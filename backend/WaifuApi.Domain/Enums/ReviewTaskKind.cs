namespace WaifuApi.Domain.Enums;

/// <summary>
/// What a review task is about.
/// </summary>
public enum ReviewTaskKind
{
    /// <summary>A newly submitted image/tag/artist awaiting first approval.</summary>
    NewContent = 0,

    /// <summary>A proposed change to an existing image/tag/artist.</summary>
    Edit = 1
}
