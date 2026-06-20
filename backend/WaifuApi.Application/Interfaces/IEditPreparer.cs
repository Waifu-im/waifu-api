using System.Threading;
using System.Threading.Tasks;
using WaifuApi.Application.Common.Models;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Interfaces;

/// <summary>
/// Validates and normalizes a proposed edit against the current target entity, producing the stored payload
/// and a hydrated target DTO. Shared by the submit-edit and edit-task flows.
/// </summary>
public interface IEditPreparer
{
    Task<PreparedEdit> PrepareAsync(ReviewableContentType type, long targetId, string payloadJson, CancellationToken cancellationToken);
}

/// <param name="NormalizedPayload">The cleaned, whitelisted payload JSON to store.</param>
/// <param name="Target">The current state of the target, for diff rendering.</param>
public sealed record PreparedEdit(string NormalizedPayload, ReviewTaskTargetDto Target);
