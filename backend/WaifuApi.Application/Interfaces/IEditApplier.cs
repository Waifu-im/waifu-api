using System.Threading;
using System.Threading.Tasks;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Interfaces;

/// <summary>
/// Applies an Edit-kind review task's proposed changes to its target entity by funnelling them through
/// the existing Update commands (UpdateImage/UpdateTag/UpdateArtist). Centralised so the approve flow and the
/// trusted-user auto-apply flow share identical, fully-validated logic.
/// </summary>
public interface IEditApplier
{
    /// <summary>
    /// Merges the task's payload over the target's current values and persists the change — the proposed
    /// value overwrites whatever the field currently holds. Throws
    /// <see cref="System.Collections.Generic.KeyNotFoundException"/> if the target no longer exists, and
    /// propagates validation exceptions from the underlying Update command. Does not change the task's status.
    /// </summary>
    Task ApplyAsync(ReviewTask task, CancellationToken cancellationToken);
}
