using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Review.ApproveReviewTask;
using WaifuApi.Application.Features.Review.CancelReviewTask;
using WaifuApi.Application.Features.Review.EditReviewTask;
using WaifuApi.Application.Features.Review.RejectReviewTask;
using WaifuApi.Application.Features.Review.SubmitEdit;
using WaifuApi.Application.Features.Tags.UpdateTag;
using WaifuApi.Domain.Enums;
using WaifuApi.Tests.Infrastructure;
using Xunit;

namespace WaifuApi.Tests;

public class ReviewFlowTests
{
    // ---- edit submission ----------------------------------------------------

    [Fact]
    public async Task SubmitEdit_AsRegularUser_QueuesPending_AndDoesNotApply()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.User);
        var tagId = await harness.SeedTagAsync("Original", "DescA");

        var dto = await harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Tag, tagId, "please fix", """{"name":"Proposed"}"""));

        Assert.Equal(ReviewTaskKind.Edit, dto.Kind);
        Assert.Equal(ReviewTaskStatus.Pending, dto.Status);

        var tag = await harness.ReadAsync(db => db.Tags.FindAsync(tagId).AsTask());
        Assert.Equal("Original", tag!.Name);
    }

    [Fact]
    public async Task SubmitEdit_AsTrustedUser_AutoApplies()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.TrustedUser);
        var tagId = await harness.SeedTagAsync("Original", "DescA");

        var dto = await harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.TrustedUser, ReviewableContentType.Tag, tagId, null, """{"name":"Proposed"}"""));

        Assert.Equal(ReviewTaskStatus.Approved, dto.Status);

        var tag = await harness.ReadAsync(db => db.Tags.FindAsync(tagId).AsTask());
        Assert.Equal("Proposed", tag!.Name);
    }

    [Fact]
    public async Task SubmitEdit_WithUnknownField_IsRejected()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.User);
        var tagId = await harness.SeedTagAsync("Original", "DescA");

        await Assert.ThrowsAsync<ArgumentException>(() => harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Tag, tagId, null, """{"name":"X","bogus":123}""")));
    }

    [Fact]
    public async Task SubmitEdit_WithNoEffectiveChange_IsRejected()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.User);
        var tagId = await harness.SeedTagAsync("Original", "DescA");

        await Assert.ThrowsAsync<ArgumentException>(() => harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Tag, tagId, null, """{"name":"Original"}""")));
    }

    [Fact]
    public async Task SubmitEdit_SecondPendingOnSameTarget_IsRejected()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.User);
        var tagId = await harness.SeedTagAsync("Original", "DescA");

        await harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Tag, tagId, null, """{"name":"Proposed"}"""));

        await Assert.ThrowsAsync<ConflictException>(() => harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Tag, tagId, null, """{"name":"Another"}""")));
    }

    [Fact]
    public async Task SubmitEdit_TargetNotAccepted_IsNotFound()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.User);
        var tagId = await harness.SeedTagAsync("Hidden", "DescA", ReviewStatus.Pending);

        await Assert.ThrowsAsync<KeyNotFoundException>(() => harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Tag, tagId, null, """{"name":"Proposed"}""")));
    }

    // ---- "admin edited first" (edit kind) -----------------------------------

    [Fact]
    public async Task Approve_Edit_PreservesAConcurrentEditToADifferentField()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.User);
        var modId = await harness.SeedUserAsync(Role.Moderator);
        var tagId = await harness.SeedTagAsync("Original", "DescA");

        var dto = await harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Tag, tagId, null, """{"name":"Proposed"}"""));

        await harness.SendAsync<TagDto>(new UpdateTagCommand(tagId, "Original", "DescB", null, null, null));

        await harness.SendAsync<Unit>(new ApproveReviewTaskCommand(dto.Id, modId));

        var tag = await harness.ReadAsync(db => db.Tags.FindAsync(tagId).AsTask());
        Assert.Equal("Proposed", tag!.Name);
        Assert.Equal("DescB", tag.Description);

        var task = await harness.ReadAsync(db => db.ReviewTasks.FindAsync(dto.Id).AsTask());
        Assert.Equal(ReviewTaskStatus.Approved, task!.Status);
        Assert.Equal(modId, task.ReviewerId);
    }

    [Fact]
    public async Task Approve_Edit_AfterFieldChangedSinceSubmission_OverwritesWithProposedValue()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.User);
        var modId = await harness.SeedUserAsync(Role.Moderator);
        var tagId = await harness.SeedTagAsync("Original", "DescA");

        var dto = await harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Tag, tagId, null, """{"name":"Proposed"}"""));

        // The field drifts after submission — approval no longer detects a conflict, it simply overwrites.
        await harness.SendAsync<TagDto>(new UpdateTagCommand(tagId, "AdminName", "DescA", null, null, null));

        await harness.SendAsync<Unit>(new ApproveReviewTaskCommand(dto.Id, modId));

        var tag = await harness.ReadAsync(db => db.Tags.FindAsync(tagId).AsTask());
        Assert.Equal("Proposed", tag!.Name);
        var task = await harness.ReadAsync(db => db.ReviewTasks.FindAsync(dto.Id).AsTask());
        Assert.Equal(ReviewTaskStatus.Approved, task!.Status);
    }

    // ---- resolution guards --------------------------------------------------

    [Fact]
    public async Task Approve_AlreadyResolved_IsRejected()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.User);
        var modId = await harness.SeedUserAsync(Role.Moderator);
        var tagId = await harness.SeedTagAsync("Original", "DescA");

        var dto = await harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Tag, tagId, null, """{"name":"Proposed"}"""));
        await harness.SendAsync<Unit>(new ApproveReviewTaskCommand(dto.Id, modId));

        await Assert.ThrowsAsync<ConflictException>(() => harness.SendAsync<Unit>(new ApproveReviewTaskCommand(dto.Id, modId)));
    }

    [Fact]
    public async Task Reject_Edit_SetsStatusAndNote_AndDoesNotMutateTarget()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.User);
        var modId = await harness.SeedUserAsync(Role.Moderator);
        var tagId = await harness.SeedTagAsync("Original", "DescA");

        var dto = await harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Tag, tagId, null, """{"name":"Proposed"}"""));

        await harness.SendAsync<Unit>(new RejectReviewTaskCommand(dto.Id, modId, "Not accurate"));

        var task = await harness.ReadAsync(db => db.ReviewTasks.FindAsync(dto.Id).AsTask());
        Assert.Equal(ReviewTaskStatus.Rejected, task!.Status);
        Assert.Equal("Not accurate", task.ReviewerNote);

        var tag = await harness.ReadAsync(db => db.Tags.FindAsync(tagId).AsTask());
        Assert.Equal("Original", tag!.Name);
    }

    [Fact]
    public async Task Cancel_Edit_OnlyOwnerOrModerator_AndOnlyWhilePending()
    {
        using var harness = new TestHarness();
        var ownerId = await harness.SeedUserAsync(Role.User);
        var strangerId = await harness.SeedUserAsync(Role.User);
        var tagId = await harness.SeedTagAsync("Original", "DescA");

        var dto = await harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            ownerId, Role.User, ReviewableContentType.Tag, tagId, null, """{"name":"Proposed"}"""));

        await Assert.ThrowsAsync<ForbiddenException>(() => harness.SendAsync<Unit>(
            new CancelReviewTaskCommand(dto.Id, strangerId, ActorIsModerator: false)));

        await harness.SendAsync<Unit>(new CancelReviewTaskCommand(dto.Id, ownerId, ActorIsModerator: false));
        var task = await harness.ReadAsync(db => db.ReviewTasks.FindAsync(dto.Id).AsTask());
        Assert.Equal(ReviewTaskStatus.Cancelled, task!.Status);

        await Assert.ThrowsAsync<ConflictException>(() => harness.SendAsync<Unit>(
            new CancelReviewTaskCommand(dto.Id, ownerId, ActorIsModerator: false)));
    }

    [Fact]
    public async Task Approve_Edit_ImageTagDelta_MergesWithCurrentAndPreservesUploaderAndStatus()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.User);
        var modId = await harness.SeedUserAsync(Role.Moderator);
        var uploaderId = await harness.SeedUserAsync(Role.User);
        var tagA = await harness.SeedTagAsync("alpha", "a");
        await harness.SeedTagAsync("beta", "b");
        var imageId = await harness.SeedImageAsync(uploaderId: uploaderId, tagIds: new List<long> { tagA });

        var dto = await harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Image, imageId, null, """{"addTagSlugs":["beta"]}"""));

        await harness.SendAsync<Unit>(new ApproveReviewTaskCommand(dto.Id, modId));

        var image = await harness.ReadAsync(db => db.Images.Include(i => i.Tags).FirstAsync(i => i.Id == imageId));

        var slugs = image.Tags.Select(t => t.Slug).OrderBy(s => s).ToArray();
        Assert.Equal(new[] { "alpha", "beta" }, slugs);
        Assert.Equal(uploaderId, image.UploaderId);
        Assert.Equal(ReviewStatus.Accepted, image.ReviewStatus);
    }

    // ---- new content --------------------------------------------------------

    [Fact]
    public async Task Approve_NewContent_PublishesEntity_AndMarksApproved()
    {
        using var harness = new TestHarness();
        var uploaderId = await harness.SeedUserAsync(Role.User);
        var modId = await harness.SeedUserAsync(Role.Moderator);
        var imageId = await harness.SeedImageAsync(uploaderId: uploaderId, status: ReviewStatus.Pending);
        var taskId = await harness.SeedNewContentTaskAsync(ReviewableContentType.Image, imageId, uploaderId);

        await harness.SendAsync<Unit>(new ApproveReviewTaskCommand(taskId, modId));

        var image = await harness.ReadAsync(db => db.Images.FindAsync(imageId).AsTask());
        Assert.Equal(ReviewStatus.Accepted, image!.ReviewStatus);
        Assert.Equal(uploaderId, image.UploaderId); // attribution preserved

        var task = await harness.ReadAsync(db => db.ReviewTasks.FindAsync(taskId).AsTask());
        Assert.Equal(ReviewTaskStatus.Approved, task!.Status);
        Assert.Equal(modId, task.ReviewerId);
    }

    [Fact]
    public async Task Reject_NewContent_DeletesEntity_ButTaskSurvivesAsRecord()
    {
        using var harness = new TestHarness();
        var uploaderId = await harness.SeedUserAsync(Role.User);
        var modId = await harness.SeedUserAsync(Role.Moderator);
        var imageId = await harness.SeedImageAsync(uploaderId: uploaderId, status: ReviewStatus.Pending);
        var taskId = await harness.SeedNewContentTaskAsync(ReviewableContentType.Image, imageId, uploaderId);

        await harness.SendAsync<Unit>(new RejectReviewTaskCommand(taskId, modId, "Low quality"));

        var image = await harness.ReadAsync(db => db.Images.FindAsync(imageId).AsTask());
        Assert.Null(image); // entity deleted

        var task = await harness.ReadAsync(db => db.ReviewTasks.FindAsync(taskId).AsTask());
        Assert.NotNull(task); // durable record survives
        Assert.Equal(ReviewTaskStatus.Rejected, task!.Status);
        Assert.Equal("Low quality", task.ReviewerNote);
        Assert.Equal(uploaderId, task.SubmitterId); // who submitted it is still recorded
    }

    [Fact]
    public async Task Cancel_NewContent_ByOwner_WithdrawsTheUpload()
    {
        using var harness = new TestHarness();
        var ownerId = await harness.SeedUserAsync(Role.User);
        var imageId = await harness.SeedImageAsync(uploaderId: ownerId, status: ReviewStatus.Pending);
        var taskId = await harness.SeedNewContentTaskAsync(ReviewableContentType.Image, imageId, ownerId);

        await harness.SendAsync<Unit>(new CancelReviewTaskCommand(taskId, ownerId, ActorIsModerator: false));

        var image = await harness.ReadAsync(db => db.Images.FindAsync(imageId).AsTask());
        Assert.Null(image);

        var task = await harness.ReadAsync(db => db.ReviewTasks.FindAsync(taskId).AsTask());
        Assert.Equal(ReviewTaskStatus.Cancelled, task!.Status);
    }

    // ---- editing a pending submission ---------------------------------------

    [Fact]
    public async Task EditTask_NewContent_ByOwner_UpdatesEntityContent_AndStaysPending_NoModCue()
    {
        using var harness = new TestHarness();
        var ownerId = await harness.SeedUserAsync(Role.User);
        var tagA = await harness.SeedTagAsync("alpha", "a");
        await harness.SeedTagAsync("beta", "b");
        var imageId = await harness.SeedImageAsync(uploaderId: ownerId, status: ReviewStatus.Pending, tagIds: new List<long> { tagA });
        var taskId = await harness.SeedNewContentTaskAsync(ReviewableContentType.Image, imageId, ownerId);

        await harness.SendAsync<ReviewTaskDto>(new EditReviewTaskCommand(
            taskId, ownerId, ActorIsModerator: false,
            """{"source":"https://example.com/x","isNsfw":true,"tags":["beta"],"artists":[]}"""));

        var image = await harness.ReadAsync(db => db.Images.Include(i => i.Tags).FirstAsync(i => i.Id == imageId));
        Assert.Equal("https://example.com/x", image.Source);
        Assert.True(image.IsNsfw);
        Assert.Equal(new[] { "beta" }, image.Tags.Select(t => t.Slug).ToArray());
        Assert.Equal(ReviewStatus.Pending, image.ReviewStatus); // still pending — editing isn't approving
        Assert.Equal(ownerId, image.UploaderId);                // uploader preserved

        var task = await harness.ReadAsync(db => db.ReviewTasks.FindAsync(taskId).AsTask());
        Assert.Equal(ReviewTaskStatus.Pending, task!.Status);
        Assert.Null(task.ModeratorEditedAt); // owner edited their own — no cue
    }

    [Fact]
    public async Task EditTask_ByModerator_SetsModeratorEditedCue()
    {
        using var harness = new TestHarness();
        var ownerId = await harness.SeedUserAsync(Role.User);
        var modId = await harness.SeedUserAsync(Role.Moderator);
        var imageId = await harness.SeedImageAsync(uploaderId: ownerId, status: ReviewStatus.Pending);
        var taskId = await harness.SeedNewContentTaskAsync(ReviewableContentType.Image, imageId, ownerId);

        await harness.SendAsync<ReviewTaskDto>(new EditReviewTaskCommand(
            taskId, modId, ActorIsModerator: true, """{"isNsfw":true,"tags":[],"artists":[]}"""));

        var task = await harness.ReadAsync(db => db.ReviewTasks.FindAsync(taskId).AsTask());
        Assert.NotNull(task!.ModeratorEditedAt); // a moderator edited someone else's submission
    }

    [Fact]
    public async Task EditTask_ByStranger_IsForbidden()
    {
        using var harness = new TestHarness();
        var ownerId = await harness.SeedUserAsync(Role.User);
        var strangerId = await harness.SeedUserAsync(Role.User);
        var imageId = await harness.SeedImageAsync(uploaderId: ownerId, status: ReviewStatus.Pending);
        var taskId = await harness.SeedNewContentTaskAsync(ReviewableContentType.Image, imageId, ownerId);

        await Assert.ThrowsAsync<ForbiddenException>(() => harness.SendAsync<ReviewTaskDto>(
            new EditReviewTaskCommand(taskId, strangerId, ActorIsModerator: false, """{"isNsfw":true,"tags":[],"artists":[]}""")));
    }

    [Fact]
    public async Task EditTask_OnResolvedTask_IsRejected()
    {
        using var harness = new TestHarness();
        var ownerId = await harness.SeedUserAsync(Role.User);
        var modId = await harness.SeedUserAsync(Role.Moderator);
        var imageId = await harness.SeedImageAsync(uploaderId: ownerId, status: ReviewStatus.Pending);
        var taskId = await harness.SeedNewContentTaskAsync(ReviewableContentType.Image, imageId, ownerId);

        await harness.SendAsync<Unit>(new ApproveReviewTaskCommand(taskId, modId));

        await Assert.ThrowsAsync<ConflictException>(() => harness.SendAsync<ReviewTaskDto>(
            new EditReviewTaskCommand(taskId, ownerId, ActorIsModerator: false, """{"isNsfw":true,"tags":[],"artists":[]}""")));
    }

    [Fact]
    public async Task EditTask_Edit_UpdatesProposedPayload()
    {
        using var harness = new TestHarness();
        var userId = await harness.SeedUserAsync(Role.User);
        var tagId = await harness.SeedTagAsync("Original", "DescA");

        var dto = await harness.SendAsync<ReviewTaskDto>(new SubmitEditCommand(
            userId, Role.User, ReviewableContentType.Tag, tagId, null, """{"name":"Proposed"}"""));

        await harness.SendAsync<ReviewTaskDto>(new EditReviewTaskCommand(
            dto.Id, userId, ActorIsModerator: false, """{"name":"Proposed2"}"""));

        // The task's stored payload now reflects the refined proposal; the target tag is untouched (still pending review).
        var task = await harness.ReadAsync(db => db.ReviewTasks.FindAsync(dto.Id).AsTask());
        Assert.Contains("Proposed2", task!.Payload);
        var tag = await harness.ReadAsync(db => db.Tags.FindAsync(tagId).AsTask());
        Assert.Equal("Original", tag!.Name);
    }
}
