using FluentValidation;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Constants;

namespace WaifuApi.Application.Features.Images.UploadImage;

public class UploadImageCommandValidator : AbstractValidator<UploadImageCommand>
{
    public UploadImageCommandValidator(IConfiguration configuration)
    {
        var maxUploadSizeMb = int.TryParse(configuration[ConfigurationKeys.Image.MaxUploadSizeMb], out var mb) ? mb : 10;
        var maxUploadSizeBytes = maxUploadSizeMb * 1024L * 1024L;

        RuleFor(x => x.UserId)
            .GreaterThan(0).WithMessage("Invalid User ID.");

        RuleFor(x => x.FileStream)
            .NotNull().WithMessage("File stream is required.")
            .Must(stream => stream != null && stream.Length <= maxUploadSizeBytes)
            .WithMessage($"File size must not exceed {maxUploadSizeMb}MB.");

        RuleFor(x => x.FileName)
            .NotEmpty().WithMessage("File name is required.");

        RuleFor(x => x.ContentType)
            .NotEmpty().WithMessage("Content type is required.")
            .Must(x => x == "image/jpeg" || x == "image/png" || x == "image/gif" || x == "image/webp")
            .WithMessage("Only JPEG, PNG, GIF, and WEBP formats are supported.");

        RuleFor(x => x.Source)
            .MaximumLength(500).WithMessage("Source URL must not exceed 500 characters.")
            .Must(BeAValidUrl).When(x => !string.IsNullOrEmpty(x.Source))
            .WithMessage("Source must be a valid URL.");
    }

    private bool BeAValidUrl(string url)
    {
        return Uri.TryCreate(url, UriKind.Absolute, out _);
    }
}
