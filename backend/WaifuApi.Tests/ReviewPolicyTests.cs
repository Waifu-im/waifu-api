using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Services;
using WaifuApi.Domain.Enums;
using Xunit;

namespace WaifuApi.Tests;

public class ReviewPolicyTests
{
    private static ReviewPolicy Policy(params (string key, string val)[] settings)
    {
        var dict = settings.ToDictionary(s => s.key, s => (string?)s.val);
        var config = new ConfigurationBuilder().AddInMemoryCollection(dict).Build();
        return new ReviewPolicy(config);
    }

    [Fact]
    public void NewContent_MasterOff_NeverRequiresReview()
    {
        var p = Policy(("Review:RequireImageReview", "false"));
        Assert.False(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.User));
        Assert.False(p.RequiresReviewForNewContent(ReviewableContentType.Image, null));
        Assert.False(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.Admin));
    }

    [Fact]
    public void NewContent_MasterOn_UserReviewed_TrustedAndAboveBypassByDefault()
    {
        var p = Policy(("Review:RequireImageReview", "true"));
        Assert.True(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.User));
        Assert.False(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.TrustedUser));
        Assert.False(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.Moderator));
        Assert.False(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.Admin));
    }

    [Fact]
    public void NewContent_NullRole_IsReviewed()
    {
        var p = Policy(("Review:RequireTagReview", "true"));
        Assert.True(p.RequiresReviewForNewContent(ReviewableContentType.Tag, null));
    }

    [Fact]
    public void NewContent_MissingMasterSwitch_DefaultsToRequireReview()
    {
        var p = Policy(); // nothing configured
        Assert.True(p.RequiresReviewForNewContent(ReviewableContentType.Artist, Role.User));
    }

    [Fact]
    public void Bypass_None_DisablesBypass_EvenForAdmin()
    {
        var p = Policy(
            ("Review:RequireImageReview", "true"),
            ("Review:ImageReviewBypassMinRole", "None"));
        Assert.True(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.TrustedUser));
        Assert.True(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.Admin));
    }

    [Fact]
    public void Bypass_RaisedToModerator_TrustedStillReviewed()
    {
        var p = Policy(
            ("Review:RequireImageReview", "true"),
            ("Review:ImageReviewBypassMinRole", "Moderator"));
        Assert.True(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.TrustedUser));
        Assert.False(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.Moderator));
        Assert.False(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.Admin));
    }

    [Fact]
    public void Bypass_IsPerContentType_Independent()
    {
        var p = Policy(
            ("Review:RequireImageReview", "true"),
            ("Review:ImageReviewBypassMinRole", "None"),
            ("Review:RequireTagReview", "true"));
        Assert.True(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.TrustedUser)); // disabled for images
        Assert.False(p.RequiresReviewForNewContent(ReviewableContentType.Tag, Role.TrustedUser));  // default bypass for tags
    }

    [Fact]
    public void EditRequest_UsesOwnMasterSwitch()
    {
        var reviewed = Policy(("Review:RequireEditRequestReview", "true"));
        Assert.True(reviewed.RequiresReviewForEditRequest(ReviewableContentType.Tag, Role.User));

        var notReviewed = Policy(("Review:RequireEditRequestReview", "false"));
        Assert.False(notReviewed.RequiresReviewForEditRequest(ReviewableContentType.Tag, Role.User));
    }

    [Fact]
    public void EditRequest_TrustedUserBypassesByDefault()
    {
        var p = Policy(("Review:RequireEditRequestReview", "true"));
        Assert.True(p.RequiresReviewForEditRequest(ReviewableContentType.Tag, Role.User));
        Assert.False(p.RequiresReviewForEditRequest(ReviewableContentType.Tag, Role.TrustedUser));
        Assert.False(p.RequiresReviewForEditRequest(ReviewableContentType.Tag, Role.Moderator));
    }

    [Fact]
    public void EditBypass_IsConfiguredIndependentlyFromCreationBypass()
    {
        // Image uploads: nobody bypasses (reviewed). Image edit requests: trusted users bypass (auto-applied).
        var p = Policy(
            ("Review:RequireImageReview", "true"),
            ("Review:ImageReviewBypassMinRole", "None"),
            ("Review:RequireEditRequestReview", "true"),
            ("Review:ImageEditReviewBypassMinRole", "TrustedUser"));

        Assert.True(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.TrustedUser)); // upload reviewed
        Assert.True(p.RequiresReviewForNewContent(ReviewableContentType.Image, Role.Admin));       // upload reviewed
        Assert.False(p.RequiresReviewForEditRequest(ReviewableContentType.Image, Role.TrustedUser)); // edit auto-applied
    }

    [Fact]
    public void CreationBypass_DoesNotLeakIntoEditBypass()
    {
        // Only the creation bypass is raised; edit requests must keep their own (default) threshold.
        var p = Policy(
            ("Review:RequireEditRequestReview", "true"),
            ("Review:TagReviewBypassMinRole", "Moderator")); // creation bypass raised, edit key absent => default TrustedUser
        Assert.True(p.RequiresReviewForNewContent(ReviewableContentType.Tag, Role.TrustedUser));    // creation now reviewed for trusted
        Assert.False(p.RequiresReviewForEditRequest(ReviewableContentType.Tag, Role.TrustedUser));  // edit unaffected (default bypass)
    }
}
