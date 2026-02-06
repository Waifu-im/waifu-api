using System;
using System.IO;
using System.Net.Http;
using System.Threading.Tasks;
using Amazon;
using Amazon.Runtime;
using Amazon.S3;
using Amazon.S3.Model;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Infrastructure.Services;

public class S3Service : IStorageService
{
    private readonly IAmazonS3 _s3Client;
    private readonly string _bucketName;

    public S3Service(IConfiguration configuration)
    {
        var accessKey = configuration["S3:AccessKey"];
        var secretKey = configuration["S3:SecretKey"];
        var serviceUrl = configuration["S3:ServiceUrl"];
        var region = configuration["S3:Region"];
        _bucketName = configuration["S3:BucketName"]!;
        
        if (string.IsNullOrEmpty(accessKey) || string.IsNullOrEmpty(secretKey) || string.IsNullOrEmpty(serviceUrl) || string.IsNullOrEmpty(_bucketName) || string.IsNullOrEmpty(region))
        {
            throw new InvalidOperationException("S3 configuration is incomplete. AccessKey, SecretKey, ServiceUrl, BucketName, and Region are required.");
        }

        var forcePathStyle = configuration.GetValue<bool>("S3:ForcePathStyle", false);
        var verifySsl = configuration.GetValue<bool>("S3:VerifySsl", true);

        var config = new AmazonS3Config
        {
            ServiceURL = serviceUrl,
            AuthenticationRegion = region,
            ForcePathStyle = forcePathStyle,
        };

        if (!verifySsl)
        {
            config.HttpClientFactory = new UntrustedCertClientFactory();
        }

        _s3Client = new AmazonS3Client(accessKey, secretKey, config);
    }

    public async Task UploadAsync(Stream stream, string fileName, string contentType)
    {
        var request = new PutObjectRequest
        {
            BucketName = _bucketName,
            Key = fileName,
            InputStream = stream,
            ContentType = contentType,
            AutoCloseStream = false,
            CannedACL = S3CannedACL.PublicRead
        };

        await _s3Client.PutObjectAsync(request);
    }

    public async Task DeleteAsync(string fileName)
    {
        await _s3Client.DeleteObjectAsync(_bucketName, fileName);
    }

    private class UntrustedCertClientFactory : HttpClientFactory
    {
        public override HttpClient CreateHttpClient(IClientConfig clientConfig)
        {
            var handler = new HttpClientHandler
            {
                ServerCertificateCustomValidationCallback = (message, cert, chain, errors) => true
            };
            return new HttpClient(handler);
        }
    }
}
