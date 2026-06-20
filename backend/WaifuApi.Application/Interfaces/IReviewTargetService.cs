using System.Threading;
using System.Threading.Tasks;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Interfaces;

/// <summary>
/// Resolves the *target entity* of a NewContent review task: publishing it on approval, or deleting it
/// (and its stored file) on rejection/cancellation. Deletion funnels through the existing Delete commands.
/// </summary>
public interface IReviewTargetService
{
    /// <summary>Publish a NewContent target (set its ReviewStatus to Accepted). Does not call SaveChanges — the caller persists within its transaction.</summary>
    Task AcceptAsync(ReviewableContentType type, long targetId, CancellationToken cancellationToken);

    /// <summary>Delete a NewContent target (and its S3 object for images). Used on reject/cancel.</summary>
    Task DeleteAsync(ReviewableContentType type, long targetId, long actorId, bool actorIsModerator, CancellationToken cancellationToken);
}
