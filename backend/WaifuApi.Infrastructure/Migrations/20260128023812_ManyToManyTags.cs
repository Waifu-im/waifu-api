using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable

namespace WaifuApi.Infrastructure.Migrations
{
    /// <inheritdoc />
    public partial class ManyToManyTags : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropForeignKey(
                name: "FK_Tags_Images_ImageId",
                table: "Tags");

            migrationBuilder.DropIndex(
                name: "IX_Tags_ImageId",
                table: "Tags");

            migrationBuilder.DropColumn(
                name: "ImageId",
                table: "Tags");

            migrationBuilder.CreateTable(
                name: "ImageTag",
                columns: table => new
                {
                    ImagesId = table.Column<long>(type: "bigint", nullable: false),
                    TagsId = table.Column<long>(type: "bigint", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_ImageTag", x => new { x.ImagesId, x.TagsId });
                    table.ForeignKey(
                        name: "FK_ImageTag_Images_ImagesId",
                        column: x => x.ImagesId,
                        principalTable: "Images",
                        principalColumn: "Id",
                        onDelete: ReferentialAction.Cascade);
                    table.ForeignKey(
                        name: "FK_ImageTag_Tags_TagsId",
                        column: x => x.TagsId,
                        principalTable: "Tags",
                        principalColumn: "Id",
                        onDelete: ReferentialAction.Cascade);
                });

            migrationBuilder.CreateIndex(
                name: "IX_ImageTag_TagsId",
                table: "ImageTag",
                column: "TagsId");
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropTable(
                name: "ImageTag");

            migrationBuilder.AddColumn<long>(
                name: "ImageId",
                table: "Tags",
                type: "bigint",
                nullable: true);

            migrationBuilder.CreateIndex(
                name: "IX_Tags_ImageId",
                table: "Tags",
                column: "ImageId");

            migrationBuilder.AddForeignKey(
                name: "FK_Tags_Images_ImageId",
                table: "Tags",
                column: "ImageId",
                principalTable: "Images",
                principalColumn: "Id");
        }
    }
}
