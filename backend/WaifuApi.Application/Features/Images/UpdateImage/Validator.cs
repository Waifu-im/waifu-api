using System.Linq;
using FluentValidation;

namespace WaifuApi.Application.Features.Images.UpdateImage;

public class UpdateImageCommandValidator : AbstractValidator<UpdateImageCommand>
{
    private static readonly string[] AllowedContentTypes = { "image/jpeg", "image/png", "image/gif", "image/webp" };

    public UpdateImageCommandValidator()
    {
        RuleFor(x => x.Id)
            .GreaterThan(0).WithMessage("Invalid Image ID.");

        RuleFor(x => x.Source)
            .MaximumLength(500).WithMessage("Source URL must not exceed 500 characters.")
            .Must(BeAValidUrl).When(x => !string.IsNullOrEmpty(x.Source))
            .WithMessage("Source must be a valid URL.");

        RuleFor(x => x.UserId)
            .GreaterThan(0).When(x => x.UserId.HasValue).WithMessage("Invalid User ID.");

        // File validation (only when file is provided)
        RuleFor(x => x.FileName)
            .NotEmpty().When(x => x.FileStream != null)
            .WithMessage("File name is required when uploading a file.");

        RuleFor(x => x.ContentType)
            .NotEmpty().When(x => x.FileStream != null)
            .WithMessage("Content type is required when uploading a file.")
            .Must(BeAValidContentType).When(x => x.FileStream != null && !string.IsNullOrEmpty(x.ContentType))
            .WithMessage($"Content type must be one of: {string.Join(", ", AllowedContentTypes)}");
    }

    private bool BeAValidUrl(string url)
    {
        return Uri.TryCreate(url, UriKind.Absolute, out _);
    }

    private bool BeAValidContentType(string? contentType)
    {
        return contentType != null && AllowedContentTypes.Contains(contentType);
    }
}