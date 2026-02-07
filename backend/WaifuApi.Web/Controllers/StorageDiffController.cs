using System.Collections.Generic;
using System.ComponentModel;
using System.ComponentModel.DataAnnotations;
using System.Threading.Tasks;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Images.BulkDeleteImages;
using WaifuApi.Application.Features.Images.DeleteS3Objects;
using WaifuApi.Application.Features.Images.GetStorageDiff;
using WaifuApi.Web.Constants;
using WaifuApi.Web.Models;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// Compare S3 storage with database records to find inconsistencies.
/// </summary>
/// <remarks>
/// This controller provides endpoints to detect orphaned S3 files (no matching DB record)
/// and missing S3 files (DB record exists but file is absent), and to clean them up.
/// All endpoints require Admin privileges.
/// </remarks>
[ApiController]
[Route("storage-diff")]
[Produces("application/json")]
[Tags("Storage Diff")]
[Authorize(Policy = AuthorizationPolicies.Admin)]
public class StorageDiffController : ControllerBase
{
    private readonly IMediator _mediator;

    public StorageDiffController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Get storage diff between S3 and database.
    /// </summary>
    /// <remarks>
    /// Returns a paginated list of discrepancies between S3 storage and the database.
    ///
    /// **Modes:**
    /// - `OrphansInS3`: Files that exist in S3 but have no matching database record.
    /// - `MissingFromS3`: Database records whose files are missing from S3.
    /// </remarks>
    /// <param name="request">Query parameters including mode and pagination.</param>
    /// <returns>A paginated list of storage diff entries.</returns>
    /// <response code="200">Returns the diff results.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Admin role required.</response>
    [HttpGet]
    [ProducesResponseType(typeof(PaginatedList<StorageDiffDto>), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    public async Task<ActionResult<PaginatedList<StorageDiffDto>>> GetDiff([FromQuery] GetStorageDiffRequest request)
    {
        var query = new GetStorageDiffQuery
        {
            Mode = request.Mode,
            Page = request.Page,
            PageSize = request.PageSize
        };
        var result = await _mediator.Send(query);
        return Ok(result);
    }

    /// <summary>
    /// Delete orphaned files from S3.
    /// </summary>
    /// <remarks>
    /// Removes one or more files from S3 storage that have no corresponding database record.
    /// Use this to clean up orphaned files found via the `OrphansInS3` diff mode.
    /// </remarks>
    /// <param name="request">The file names to delete.</param>
    /// <response code="204">Files deleted successfully.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Admin role required.</response>
    [HttpDelete("s3")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    public async Task<IActionResult> DeleteS3Objects([FromBody] DeleteS3ObjectsRequest request)
    {
        await _mediator.Send(new DeleteS3ObjectsCommand(request.FileNames));
        return NoContent();
    }

    /// <summary>
    /// Delete database records for images missing from S3.
    /// </summary>
    /// <remarks>
    /// Removes one or more image records from the database whose files no longer exist in S3.
    /// Use this to clean up orphaned database entries found via the `MissingFromS3` diff mode.
    /// No S3 deletion is performed since the files are already absent.
    /// </remarks>
    /// <param name="request">The image IDs to delete.</param>
    /// <response code="204">Records deleted successfully.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Admin role required.</response>
    [HttpDelete("db")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    public async Task<IActionResult> DeleteDbRecords([FromBody] DeleteDbRecordsRequest request)
    {
        await _mediator.Send(new BulkDeleteImagesCommand(request.ImageIds));
        return NoContent();
    }
}

/// <summary>
/// Request model for querying storage diff between S3 and database.
/// </summary>
public class GetStorageDiffRequest
{
    /// <summary>
    /// The diff mode: OrphansInS3 (files in S3 without DB record) or MissingFromS3 (DB records without S3 file).
    /// </summary>
    [Required]
    [Description("The diff mode: OrphansInS3 or MissingFromS3.")]
    public StorageDiffMode Mode { get; set; }

    /// <summary>
    /// Page number for pagination. Default: 1.
    /// </summary>
    [Description("Page number for pagination. Default: 1.")]
    public int Page { get; set; } = 1;

    /// <summary>
    /// Number of results per page.
    /// </summary>
    [Description("Number of results per page.")]
    public int PageSize { get; set; }
}

/// <summary>
/// Request body for deleting orphaned S3 files.
/// </summary>
public class DeleteS3ObjectsRequest
{
    /// <summary>
    /// List of S3 file names (keys) to delete.
    /// </summary>
    [Required]
    public List<string> FileNames { get; set; } = new();
}

/// <summary>
/// Request body for deleting orphaned database records.
/// </summary>
public class DeleteDbRecordsRequest
{
    /// <summary>
    /// List of image IDs to delete from the database.
    /// </summary>
    [Required]
    public List<long> ImageIds { get; set; } = new();
}
